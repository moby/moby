package dockerfile

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	apierrors "github.com/docker/docker/api/errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
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

// Builder is a Dockerfile builder
// It implements the builder.Backend interface.
type Builder struct {
	options *types.ImageBuildOptions

	Stdout io.Writer
	Stderr io.Writer
	Output io.Writer

	docker    builder.Backend
	context   builder.Context
	clientCtx context.Context

	runConfig     *container.Config // runconfig for cmd, run, entrypoint etc.
	flags         *BFlags
	tmpContainers map[string]struct{}
	image         string         // imageID
	imageContexts *imageContexts // helper for storing contexts from builds
	noBaseImage   bool           // A flag to track the use of `scratch` as the base image
	maintainer    string
	cmdSet        bool
	disableCommit bool
	cacheBusted   bool
	buildArgs     *buildArgs
	escapeToken   rune

	imageCache builder.ImageCache
	from       builder.Image
}

// BuildManager implements builder.Backend and is shared across all Builder objects.
type BuildManager struct {
	backend   builder.Backend
	pathCache *pathCache // TODO: make this persistent
}

// NewBuildManager creates a BuildManager.
func NewBuildManager(b builder.Backend) (bm *BuildManager) {
	return &BuildManager{backend: b, pathCache: &pathCache{}}
}

// BuildFromContext builds a new image from a given context.
func (bm *BuildManager) BuildFromContext(ctx context.Context, src io.ReadCloser, remote string, buildOptions *types.ImageBuildOptions, pg backend.ProgressWriter) (string, error) {
	if buildOptions.Squash && !bm.backend.HasExperimental() {
		return "", apierrors.NewBadRequestError(errors.New("squash is only supported with experimental mode"))
	}
	buildContext, dockerfileName, err := builder.DetectContextFromRemoteURL(src, remote, pg.ProgressReaderFunc)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := buildContext.Close(); err != nil {
			logrus.Debugf("[BUILDER] failed to remove temporary context: %v", err)
		}
	}()

	if len(dockerfileName) > 0 {
		buildOptions.Dockerfile = dockerfileName
	}
	b, err := NewBuilder(ctx, buildOptions, bm.backend, builder.DockerIgnoreContext{ModifiableContext: buildContext})
	if err != nil {
		return "", err
	}
	b.imageContexts.cache = bm.pathCache
	return b.build(pg.StdoutFormatter, pg.StderrFormatter, pg.Output)
}

// NewBuilder creates a new Dockerfile builder from an optional dockerfile and a Config.
// If dockerfile is nil, the Dockerfile specified by Config.DockerfileName,
// will be read from the Context passed to Build().
func NewBuilder(clientCtx context.Context, config *types.ImageBuildOptions, backend builder.Backend, buildContext builder.Context) (b *Builder, err error) {
	if config == nil {
		config = new(types.ImageBuildOptions)
	}
	b = &Builder{
		clientCtx:     clientCtx,
		options:       config,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		docker:        backend,
		context:       buildContext,
		runConfig:     new(container.Config),
		tmpContainers: map[string]struct{}{},
		buildArgs:     newBuildArgs(config.BuildArgs),
		escapeToken:   parser.DefaultEscapeToken,
	}
	b.imageContexts = &imageContexts{b: b}
	return b, nil
}

func (b *Builder) resetImageCache() {
	if icb, ok := b.docker.(builder.ImageCacheBuilder); ok {
		b.imageCache = icb.MakeImageCache(b.options.CacheFrom)
	}
	b.noBaseImage = false
	b.cacheBusted = false
}

// sanitizeRepoAndTags parses the raw "t" parameter received from the client
// to a slice of repoAndTag.
// It also validates each repoName and tag.
func sanitizeRepoAndTags(names []string) ([]reference.Named, error) {
	var (
		repoAndTags []reference.Named
		// This map is used for deduplicating the "-t" parameter.
		uniqNames = make(map[string]struct{})
	)
	for _, repo := range names {
		if repo == "" {
			continue
		}

		ref, err := reference.ParseNormalizedNamed(repo)
		if err != nil {
			return nil, err
		}

		if _, isCanonical := ref.(reference.Canonical); isCanonical {
			return nil, errors.New("build tag cannot contain a digest")
		}

		ref = reference.TagNameOnly(ref)

		nameWithTag := ref.String()

		if _, exists := uniqNames[nameWithTag]; !exists {
			uniqNames[nameWithTag] = struct{}{}
			repoAndTags = append(repoAndTags, ref)
		}
	}
	return repoAndTags, nil
}

// build runs the Dockerfile builder from a context and a docker object that allows to make calls
// to Docker.
func (b *Builder) build(stdout io.Writer, stderr io.Writer, out io.Writer) (string, error) {
	defer b.imageContexts.unmount()

	b.Stdout = stdout
	b.Stderr = stderr
	b.Output = out

	dockerfile, err := b.readAndParseDockerfile()
	if err != nil {
		return "", err
	}

	repoAndTags, err := sanitizeRepoAndTags(b.options.Tags)
	if err != nil {
		return "", err
	}

	addNodesForLabelOption(dockerfile.AST, b.options.Labels)

	if err := checkDispatchDockerfile(dockerfile.AST); err != nil {
		return "", err
	}

	shortImageID, err := b.dispatchDockerfileWithCancellation(dockerfile)
	if err != nil {
		return "", err
	}

	b.warnOnUnusedBuildArgs()

	if b.image == "" {
		return "", errors.New("No image was generated. Is your Dockerfile empty?")
	}

	if b.options.Squash {
		if err := b.squashBuild(); err != nil {
			return "", err
		}
	}

	fmt.Fprintf(b.Stdout, "Successfully built %s\n", shortImageID)
	if err := b.tagImages(repoAndTags); err != nil {
		return "", err
	}
	return b.image, nil
}

func (b *Builder) dispatchDockerfileWithCancellation(dockerfile *parser.Result) (string, error) {
	// TODO: pass this to dispatchRequest instead
	b.escapeToken = dockerfile.EscapeToken

	total := len(dockerfile.AST.Children)
	var shortImgID string
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

		if err := b.dispatch(i, total, n); err != nil {
			if b.options.ForceRemove {
				b.clearTmp()
			}
			return "", err
		}

		shortImgID = stringid.TruncateID(b.image)
		fmt.Fprintf(b.Stdout, " ---> %s\n", shortImgID)
		if b.options.Remove {
			b.clearTmp()
		}
	}

	if b.options.Target != "" && !b.imageContexts.isCurrentTarget(b.options.Target) {
		return "", errors.Errorf("failed to reach build target %s in Dockerfile", b.options.Target)
	}

	return shortImgID, nil
}

func (b *Builder) squashBuild() error {
	var fromID string
	var err error
	if b.from != nil {
		fromID = b.from.ImageID()
	}
	b.image, err = b.docker.SquashImage(b.image, fromID)
	if err != nil {
		return errors.Wrap(err, "error squashing image")
	}
	return nil
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

func (b *Builder) tagImages(repoAndTags []reference.Named) error {
	imageID := image.ID(b.image)
	for _, rt := range repoAndTags {
		if err := b.docker.TagImageWithReference(imageID, rt); err != nil {
			return err
		}
		fmt.Fprintf(b.Stdout, "Successfully tagged %s\n", reference.FamiliarString(rt))
	}
	return nil
}

// hasFromImage returns true if the builder has processed a `FROM <image>` line
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
	b, err := NewBuilder(context.Background(), nil, nil, nil)
	if err != nil {
		return nil, err
	}

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
	// TODO: pass this to dispatchRequest instead
	b.escapeToken = result.EscapeToken
	ast := result.AST
	total := len(ast.Children)

	for i, n := range ast.Children {
		if err := b.dispatch(i, total, n); err != nil {
			return err
		}
	}
	return nil
}
