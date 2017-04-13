package dockerfile

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/sync/syncmap"
)

var validCommitCommands = map[string]bool{
	"cmd":         true,
	"entrypoint":  true,
	"healthcheck": true,
	"env":         true,
	"expose":      true,
	"label":       true,
	"onbuild":     true,
	"user":        true,
	"volume":      true,
	"workdir":     true,
}

var defaultLogConfig = container.LogConfig{Type: "none"}

// BuildManager is shared across all Builder objects
type BuildManager struct {
	backend   builder.Backend
	pathCache pathCache // TODO: make this persistent
}

// NewBuildManager creates a BuildManager
func NewBuildManager(b builder.Backend) *BuildManager {
	return &BuildManager{backend: b, pathCache: &syncmap.Map{}}
}

// Build starts a new build from a BuildConfig
func (bm *BuildManager) Build(ctx context.Context, config backend.BuildConfig) (*builder.Result, error) {
	if config.Options.Dockerfile == "" {
		config.Options.Dockerfile = builder.DefaultDockerfileName
	}

	source, dockerfile, err := remotecontext.Detect(config)
	if err != nil {
		return nil, err
	}
	if source != nil {
		defer func() {
			if err := source.Close(); err != nil {
				logrus.Debugf("[BUILDER] failed to remove temporary context: %v", err)
			}
		}()
	}

	builderOptions := builderOptions{
		Options:        config.Options,
		ProgressWriter: config.ProgressWriter,
		Backend:        bm.backend,
		PathCache:      bm.pathCache,
	}
	return newBuilder(ctx, builderOptions).build(source, dockerfile)
}

// builderOptions are the dependencies required by the builder
type builderOptions struct {
	Options        *types.ImageBuildOptions
	Backend        builder.Backend
	ProgressWriter backend.ProgressWriter
	PathCache      pathCache
}

// Builder is a Dockerfile builder
// It implements the builder.Backend interface.
type Builder struct {
	options *types.ImageBuildOptions

	Stdout io.Writer
	Stderr io.Writer
	Output io.Writer

	docker    builder.Backend
	source    builder.Source
	clientCtx context.Context

	runConfig     *container.Config // runconfig for cmd, run, entrypoint etc.
	tmpContainers map[string]struct{}
	imageContexts *imageContexts // helper for storing contexts from builds
	disableCommit bool
	cacheBusted   bool
	buildArgs     *buildArgs
	imageCache    builder.ImageCache

	// TODO: these move to DispatchState
	maintainer  string
	cmdSet      bool
	noBaseImage bool   // A flag to track the use of `scratch` as the base image
	image       string // imageID
	from        builder.Image
}

// newBuilder creates a new Dockerfile builder from an optional dockerfile and a Options.
func newBuilder(clientCtx context.Context, options builderOptions) *Builder {
	config := options.Options
	if config == nil {
		config = new(types.ImageBuildOptions)
	}
	b := &Builder{
		clientCtx:     clientCtx,
		options:       config,
		Stdout:        options.ProgressWriter.StdoutFormatter,
		Stderr:        options.ProgressWriter.StderrFormatter,
		Output:        options.ProgressWriter.Output,
		docker:        options.Backend,
		runConfig:     new(container.Config),
		tmpContainers: map[string]struct{}{},
		buildArgs:     newBuildArgs(config.BuildArgs),
	}
	b.imageContexts = &imageContexts{b: b, cache: options.PathCache}
	return b
}

func (b *Builder) resetImageCache() {
	if icb, ok := b.docker.(builder.ImageCacheBuilder); ok {
		b.imageCache = icb.MakeImageCache(b.options.CacheFrom)
	}
	b.noBaseImage = false
	b.cacheBusted = false
}

// Build runs the Dockerfile builder by parsing the Dockerfile and executing
// the instructions from the file.
func (b *Builder) build(source builder.Source, dockerfile *parser.Result) (*builder.Result, error) {
	defer b.imageContexts.unmount()

	// TODO: Remove source field from Builder
	b.source = source

	addNodesForLabelOption(dockerfile.AST, b.options.Labels)

	if err := checkDispatchDockerfile(dockerfile.AST); err != nil {
		return nil, err
	}

	imageID, err := b.dispatchDockerfileWithCancellation(dockerfile)
	if err != nil {
		return nil, err
	}

	b.warnOnUnusedBuildArgs()

	if imageID == "" {
		return nil, errors.New("No image was generated. Is your Dockerfile empty?")
	}
	return &builder.Result{ImageID: imageID, FromImage: b.from}, nil
}

func (b *Builder) dispatchDockerfileWithCancellation(dockerfile *parser.Result) (string, error) {
	shlex := NewShellLex(dockerfile.EscapeToken)

	total := len(dockerfile.AST.Children)
	var imageID string
	for i, n := range dockerfile.AST.Children {
		select {
		case <-b.clientCtx.Done():
			logrus.Debug("Builder: build cancelled!")
			fmt.Fprint(b.Stdout, "Build cancelled")
			return "", errors.New("Build cancelled")
		default:
			// Not cancelled yet, keep going...
		}

		if command.From == n.Value && b.imageContexts.isCurrentTarget(b.options.Target) {
			break
		}

		if err := b.dispatch(i, total, n, shlex); err != nil {
			if b.options.ForceRemove {
				b.clearTmp()
			}
			return "", err
		}

		// TODO: get this from dispatch
		imageID = b.image

		fmt.Fprintf(b.Stdout, " ---> %s\n", stringid.TruncateID(imageID))
		if b.options.Remove {
			b.clearTmp()
		}
	}

	if b.options.Target != "" && !b.imageContexts.isCurrentTarget(b.options.Target) {
		return "", errors.Errorf("failed to reach build target %s in Dockerfile", b.options.Target)
	}

	return imageID, nil
}

func addNodesForLabelOption(dockerfile *parser.Node, labels map[string]string) {
	if len(labels) == 0 {
		return
	}

	node := parser.NodeFromLabels(labels)
	dockerfile.Children = append(dockerfile.Children, node)
}

// check if there are any leftover build-args that were passed but not
// consumed during build. Print a warning, if there are any.
func (b *Builder) warnOnUnusedBuildArgs() {
	leftoverArgs := b.buildArgs.UnreferencedOptionArgs()
	if len(leftoverArgs) > 0 {
		fmt.Fprintf(b.Stderr, "[Warning] One or more build-args %v were not consumed\n", leftoverArgs)
	}
}

// hasFromImage returns true if the builder has processed a `FROM <image>` line
// TODO: move to DispatchState
func (b *Builder) hasFromImage() bool {
	return b.image != "" || b.noBaseImage
}

// BuildFromConfig builds directly from `changes`, treating it as if it were the contents of a Dockerfile
// It will:
// - Call parse.Parse() to get an AST root for the concatenated Dockerfile entries.
// - Do build by calling builder.dispatch() to call all entries' handling routines
//
// BuildFromConfig is used by the /commit endpoint, with the changes
// coming from the query parameter of the same name.
//
// TODO: Remove?
func BuildFromConfig(config *container.Config, changes []string) (*container.Config, error) {
	b := newBuilder(context.Background(), builderOptions{})

	result, err := parser.Parse(bytes.NewBufferString(strings.Join(changes, "\n")))
	if err != nil {
		return nil, err
	}

	// ensure that the commands are valid
	for _, n := range result.AST.Children {
		if !validCommitCommands[n.Value] {
			return nil, fmt.Errorf("%s is not a valid change command", n.Value)
		}
	}

	b.runConfig = config
	b.Stdout = ioutil.Discard
	b.Stderr = ioutil.Discard
	b.disableCommit = true

	if err := checkDispatchDockerfile(result.AST); err != nil {
		return nil, err
	}

	if err := dispatchFromDockerfile(b, result); err != nil {
		return nil, err
	}
	return b.runConfig, nil
}

func checkDispatchDockerfile(dockerfile *parser.Node) error {
	for _, n := range dockerfile.Children {
		if err := checkDispatch(n); err != nil {
			return errors.Wrapf(err, "Dockerfile parse error line %d", n.StartLine)
		}
	}
	return nil
}

func dispatchFromDockerfile(b *Builder, result *parser.Result) error {
	shlex := NewShellLex(result.EscapeToken)
	ast := result.AST
	total := len(ast.Children)

	for i, n := range ast.Children {
		if err := b.dispatch(i, total, n, shlex); err != nil {
			return err
		}
	}
	return nil
}
