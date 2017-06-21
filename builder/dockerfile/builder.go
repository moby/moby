package dockerfile

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/builder/fscache"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/moby/buildkit/session"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

// SessionGetter is object used to get access to a session by uuid
type SessionGetter interface {
	Get(ctx context.Context, uuid string) (session.Caller, error)
}

// BuildManager is shared across all Builder objects
type BuildManager struct {
	archiver  *archive.Archiver
	backend   builder.Backend
	pathCache pathCache // TODO: make this persistent
	sg        SessionGetter
	fsCache   *fscache.FSCache
}

// NewBuildManager creates a BuildManager
func NewBuildManager(b builder.Backend, sg SessionGetter, fsCache *fscache.FSCache, idMappings *idtools.IDMappings) (*BuildManager, error) {
	bm := &BuildManager{
		backend:   b,
		pathCache: &syncmap.Map{},
		sg:        sg,
		archiver:  chrootarchive.NewArchiver(idMappings),
		fsCache:   fsCache,
	}
	if err := fsCache.RegisterTransport(remotecontext.ClientSessionRemote, NewClientSessionTransport()); err != nil {
		return nil, err
	}
	return bm, nil
}

// Build starts a new build from a BuildConfig
func (bm *BuildManager) Build(ctx context.Context, config backend.BuildConfig) (*builder.Result, error) {
	buildsTriggered.Inc()
	if config.Options.Dockerfile == "" {
		config.Options.Dockerfile = builder.DefaultDockerfileName
	}

	source, dockerfile, err := remotecontext.Detect(config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if source != nil {
			if err := source.Close(); err != nil {
				logrus.Debugf("[BUILDER] failed to remove temporary context: %v", err)
			}
		}
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if src, err := bm.initializeClientSession(ctx, cancel, config.Options); err != nil {
		return nil, err
	} else if src != nil {
		source = src
	}

	platform := runtime.GOOS
	if dockerfile.Platform != "" {
		if config.Options.Platform != "" && config.Options.Platform != dockerfile.Platform {
			return nil, fmt.Errorf("invalid platform")
		}
		platform = dockerfile.Platform
	} else if config.Options.Platform != "" {
		platform = config.Options.Platform
	}
	config.Options.Platform = platform
	dockerfile.Platform = platform

	builderOptions := builderOptions{
		Options:        config.Options,
		ProgressWriter: config.ProgressWriter,
		Backend:        bm.backend,
		PathCache:      bm.pathCache,
		Archiver:       bm.archiver,
	}
	return newBuilder(ctx, builderOptions, platform).build(source, dockerfile)
}

func (bm *BuildManager) initializeClientSession(ctx context.Context, cancel func(), options *types.ImageBuildOptions) (builder.Source, error) {
	if options.SessionID == "" || bm.sg == nil {
		return nil, nil
	}
	logrus.Debug("client is session enabled")

	ctx, cancelCtx := context.WithTimeout(ctx, sessionConnectTimeout)
	defer cancelCtx()

	c, err := bm.sg.Get(ctx, options.SessionID)
	if err != nil {
		return nil, err
	}
	go func() {
		<-c.Context().Done()
		cancel()
	}()
	if options.RemoteContext == remotecontext.ClientSessionRemote {
		st := time.Now()
		csi, err := NewClientSessionSourceIdentifier(ctx, bm.sg, options.SessionID)
		if err != nil {
			return nil, err
		}
		src, err := bm.fsCache.SyncFrom(ctx, csi)
		if err != nil {
			return nil, err
		}
		logrus.Debugf("sync-time: %v", time.Since(st))
		return src, nil
	}
	return nil, nil
}

// builderOptions are the dependencies required by the builder
type builderOptions struct {
	Options        *types.ImageBuildOptions
	Backend        builder.Backend
	ProgressWriter backend.ProgressWriter
	PathCache      pathCache
	Archiver       *archive.Archiver
}

// Builder is a Dockerfile builder
// It implements the builder.Backend interface.
type Builder struct {
	options *types.ImageBuildOptions

	Stdout io.Writer
	Stderr io.Writer
	Aux    *streamformatter.AuxFormatter
	Output io.Writer

	docker    builder.Backend
	clientCtx context.Context

	archiver         *archive.Archiver
	buildStages      *buildStages
	disableCommit    bool
	buildArgs        *buildArgs
	imageSources     *imageSources
	pathCache        pathCache
	containerManager *containerManager
	imageProber      ImageProber
}

// newBuilder creates a new Dockerfile builder from an optional dockerfile and a Options.
func newBuilder(clientCtx context.Context, options builderOptions, platform string) *Builder {
	config := options.Options
	if config == nil {
		config = new(types.ImageBuildOptions)
	}

	b := &Builder{
		clientCtx:        clientCtx,
		options:          config,
		Stdout:           options.ProgressWriter.StdoutFormatter,
		Stderr:           options.ProgressWriter.StderrFormatter,
		Aux:              options.ProgressWriter.AuxFormatter,
		Output:           options.ProgressWriter.Output,
		docker:           options.Backend,
		archiver:         options.Archiver,
		buildArgs:        newBuildArgs(config.BuildArgs),
		buildStages:      newBuildStages(),
		imageSources:     newImageSources(clientCtx, options),
		pathCache:        options.PathCache,
		imageProber:      newImageProber(options.Backend, config.CacheFrom, platform, config.NoCache),
		containerManager: newContainerManager(options.Backend),
	}

	return b
}

// Build runs the Dockerfile builder by parsing the Dockerfile and executing
// the instructions from the file.
func (b *Builder) build(source builder.Source, dockerfile *parser.Result) (*builder.Result, error) {
	defer b.imageSources.Unmount()

	addNodesForLabelOption(dockerfile.AST, b.options.Labels)

	if err := checkDispatchDockerfile(dockerfile.AST); err != nil {
		buildsFailed.WithValues(metricsDockerfileSyntaxError).Inc()
		return nil, err
	}

	dispatchState, err := b.dispatchDockerfileWithCancellation(dockerfile, source)
	if err != nil {
		return nil, err
	}

	if b.options.Target != "" && !dispatchState.isCurrentStage(b.options.Target) {
		buildsFailed.WithValues(metricsBuildTargetNotReachableError).Inc()
		return nil, errors.Errorf("failed to reach build target %s in Dockerfile", b.options.Target)
	}

	dockerfile.PrintWarnings(b.Stderr)
	b.buildArgs.WarnOnUnusedBuildArgs(b.Stderr)

	if dispatchState.imageID == "" {
		buildsFailed.WithValues(metricsDockerfileEmptyError).Inc()
		return nil, errors.New("No image was generated. Is your Dockerfile empty?")
	}
	return &builder.Result{ImageID: dispatchState.imageID, FromImage: dispatchState.baseImage}, nil
}

func emitImageID(aux *streamformatter.AuxFormatter, state *dispatchState) error {
	if aux == nil || state.imageID == "" {
		return nil
	}
	return aux.Emit(types.BuildResult{ID: state.imageID})
}

func (b *Builder) dispatchDockerfileWithCancellation(dockerfile *parser.Result, source builder.Source) (*dispatchState, error) {
	shlex := NewShellLex(dockerfile.EscapeToken)
	state := newDispatchState()
	total := len(dockerfile.AST.Children)
	var err error
	for i, n := range dockerfile.AST.Children {
		select {
		case <-b.clientCtx.Done():
			logrus.Debug("Builder: build cancelled!")
			fmt.Fprint(b.Stdout, "Build cancelled")
			buildsFailed.WithValues(metricsBuildCanceled).Inc()
			return nil, errors.New("Build cancelled")
		default:
			// Not cancelled yet, keep going...
		}

		// If this is a FROM and we have a previous image then
		// emit an aux message for that image since it is the
		// end of the previous stage
		if n.Value == command.From {
			if err := emitImageID(b.Aux, state); err != nil {
				return nil, err
			}
		}

		if n.Value == command.From && state.isCurrentStage(b.options.Target) {
			break
		}

		opts := dispatchOptions{
			state:   state,
			stepMsg: formatStep(i, total),
			node:    n,
			shlex:   shlex,
			source:  source,
		}
		if state, err = b.dispatch(opts); err != nil {
			if b.options.ForceRemove {
				b.containerManager.RemoveAll(b.Stdout)
			}
			return nil, err
		}

		fmt.Fprintf(b.Stdout, " ---> %s\n", stringid.TruncateID(state.imageID))
		if b.options.Remove {
			b.containerManager.RemoveAll(b.Stdout)
		}
	}

	// Emit a final aux message for the final image
	if err := emitImageID(b.Aux, state); err != nil {
		return nil, err
	}

	return state, nil
}

func addNodesForLabelOption(dockerfile *parser.Node, labels map[string]string) {
	if len(labels) == 0 {
		return
	}

	node := parser.NodeFromLabels(labels)
	dockerfile.Children = append(dockerfile.Children, node)
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
	if len(changes) == 0 {
		return config, nil
	}

	dockerfile, err := parser.Parse(bytes.NewBufferString(strings.Join(changes, "\n")))
	if err != nil {
		return nil, err
	}

	platform := runtime.GOOS
	if dockerfile.Platform != "" {
		platform = dockerfile.Platform
	}

	b := newBuilder(context.Background(), builderOptions{
		Options: &types.ImageBuildOptions{NoCache: true},
	}, platform)

	// ensure that the commands are valid
	for _, n := range dockerfile.AST.Children {
		if !validCommitCommands[n.Value] {
			return nil, fmt.Errorf("%s is not a valid change command", n.Value)
		}
	}

	b.Stdout = ioutil.Discard
	b.Stderr = ioutil.Discard
	b.disableCommit = true

	if err := checkDispatchDockerfile(dockerfile.AST); err != nil {
		return nil, err
	}
	dispatchState := newDispatchState()
	dispatchState.runConfig = config
	return dispatchFromDockerfile(b, dockerfile, dispatchState, nil)
}

func checkDispatchDockerfile(dockerfile *parser.Node) error {
	for _, n := range dockerfile.Children {
		if err := checkDispatch(n); err != nil {
			return errors.Wrapf(err, "Dockerfile parse error line %d", n.StartLine)
		}
	}
	return nil
}

func dispatchFromDockerfile(b *Builder, result *parser.Result, dispatchState *dispatchState, source builder.Source) (*container.Config, error) {
	shlex := NewShellLex(result.EscapeToken)
	ast := result.AST
	total := len(ast.Children)

	for i, n := range ast.Children {
		opts := dispatchOptions{
			state:   dispatchState,
			stepMsg: formatStep(i, total),
			node:    n,
			shlex:   shlex,
			source:  source,
		}
		if _, err := b.dispatch(opts); err != nil {
			return nil, err
		}
	}
	return dispatchState.runConfig, nil
}
