package dockerfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/reference"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
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

// BuiltinAllowedBuildArgs is list of built-in allowed build args
var BuiltinAllowedBuildArgs = map[string]bool{
	"HTTP_PROXY":  true,
	"http_proxy":  true,
	"HTTPS_PROXY": true,
	"https_proxy": true,
	"FTP_PROXY":   true,
	"ftp_proxy":   true,
	"NO_PROXY":    true,
	"no_proxy":    true,
}

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
	cancel    context.CancelFunc

	dockerfile       *parser.Node
	runConfig        *container.Config // runconfig for cmd, run, entrypoint etc.
	flags            *BFlags
	tmpContainers    map[string]struct{}
	image            string // imageID
	noBaseImage      bool
	maintainer       string
	cmdSet           bool
	disableCommit    bool
	cacheBusted      bool
	allowedBuildArgs map[string]bool // list of build-time args that are allowed for expansion/substitution and passing to commands in 'run'.

	// TODO: remove once docker.Commit can receive a tag
	id string
}

// BuildManager implements builder.Backend and is shared across all Builder objects.
type BuildManager struct {
	backend builder.Backend
}

// NewBuildManager creates a BuildManager.
func NewBuildManager(b builder.Backend) (bm *BuildManager) {
	return &BuildManager{backend: b}
}

// BuildFromContext builds a new image from a given context.
func (bm *BuildManager) BuildFromContext(ctx context.Context, src io.ReadCloser, remote string, buildOptions *types.ImageBuildOptions, pg backend.ProgressWriter) (string, error) {
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
	b, err := NewBuilder(ctx, buildOptions, bm.backend, builder.DockerIgnoreContext{ModifiableContext: buildContext}, nil)
	if err != nil {
		return "", err
	}
	return b.build(pg.StdoutFormatter, pg.StderrFormatter, pg.Output)
}

// NewBuilder creates a new Dockerfile builder from an optional dockerfile and a Config.
// If dockerfile is nil, the Dockerfile specified by Config.DockerfileName,
// will be read from the Context passed to Build().
func NewBuilder(clientCtx context.Context, config *types.ImageBuildOptions, backend builder.Backend, buildContext builder.Context, dockerfile io.ReadCloser) (b *Builder, err error) {
	if config == nil {
		config = new(types.ImageBuildOptions)
	}
	if config.BuildArgs == nil {
		config.BuildArgs = make(map[string]string)
	}
	ctx, cancel := context.WithCancel(clientCtx)
	b = &Builder{
		clientCtx:        ctx,
		cancel:           cancel,
		options:          config,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
		docker:           backend,
		context:          buildContext,
		runConfig:        new(container.Config),
		tmpContainers:    map[string]struct{}{},
		id:               stringid.GenerateNonCryptoID(),
		allowedBuildArgs: make(map[string]bool),
	}
	if dockerfile != nil {
		b.dockerfile, err = parser.Parse(dockerfile)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
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

		ref, err := reference.ParseNamed(repo)
		if err != nil {
			return nil, err
		}

		ref = reference.WithDefaultTag(ref)

		if _, isCanonical := ref.(reference.Canonical); isCanonical {
			return nil, errors.New("build tag cannot contain a digest")
		}

		if _, isTagged := ref.(reference.NamedTagged); !isTagged {
			ref, err = reference.WithTag(ref, reference.DefaultTag)
			if err != nil {
				return nil, err
			}
		}

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
//
// This will (barring errors):
//
// * read the dockerfile from context
// * parse the dockerfile if not already parsed
// * walk the AST and execute it by dispatching to handlers. If Remove
//   or ForceRemove is set, additional cleanup around containers happens after
//   processing.
// * Tag image, if applicable.
// * Print a happy message and return the image ID.
//
func (b *Builder) build(stdout io.Writer, stderr io.Writer, out io.Writer) (string, error) {
	b.Stdout = stdout
	b.Stderr = stderr
	b.Output = out

	// If Dockerfile was not parsed yet, extract it from the Context
	if b.dockerfile == nil {
		if err := b.readDockerfile(); err != nil {
			return "", err
		}
	}

	repoAndTags, err := sanitizeRepoAndTags(b.options.Tags)
	if err != nil {
		return "", err
	}

	if len(b.options.Labels) > 0 {
		line := "LABEL "
		for k, v := range b.options.Labels {
			line += fmt.Sprintf("%q=%q ", k, v)
		}
		_, node, err := parser.ParseLine(line)
		if err != nil {
			return "", err
		}
		b.dockerfile.Children = append(b.dockerfile.Children, node)
	}

	var shortImgID string
	for i, n := range b.dockerfile.Children {
		select {
		case <-b.clientCtx.Done():
			logrus.Debug("Builder: build cancelled!")
			fmt.Fprintf(b.Stdout, "Build cancelled")
			return "", fmt.Errorf("Build cancelled")
		default:
			// Not cancelled yet, keep going...
		}
		if err := b.dispatch(i, n); err != nil {
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

	// check if there are any leftover build-args that were passed but not
	// consumed during build. Return an error, if there are any.
	leftoverArgs := []string{}
	for arg := range b.options.BuildArgs {
		if !b.isBuildArgAllowed(arg) {
			leftoverArgs = append(leftoverArgs, arg)
		}
	}
	if len(leftoverArgs) > 0 {
		return "", fmt.Errorf("One or more build-args %v were not consumed, failing build.", leftoverArgs)
	}

	if b.image == "" {
		return "", fmt.Errorf("No image was generated. Is your Dockerfile empty?")
	}

	imageID := image.ID(b.image)
	for _, rt := range repoAndTags {
		if err := b.docker.TagImageWithReference(imageID, rt); err != nil {
			return "", err
		}
	}

	fmt.Fprintf(b.Stdout, "Successfully built %s\n", shortImgID)
	return b.image, nil
}

// Cancel cancels an ongoing Dockerfile build.
func (b *Builder) Cancel() {
	b.cancel()
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
	ast, err := parser.Parse(bytes.NewBufferString(strings.Join(changes, "\n")))
	if err != nil {
		return nil, err
	}

	// ensure that the commands are valid
	for _, n := range ast.Children {
		if !validCommitCommands[n.Value] {
			return nil, fmt.Errorf("%s is not a valid change command", n.Value)
		}
	}

	b, err := NewBuilder(context.Background(), nil, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	b.runConfig = config
	b.Stdout = ioutil.Discard
	b.Stderr = ioutil.Discard
	b.disableCommit = true

	for i, n := range ast.Children {
		if err := b.dispatch(i, n); err != nil {
			return nil, err
		}
	}

	return b.runConfig, nil
}
