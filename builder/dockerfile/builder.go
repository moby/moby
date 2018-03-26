package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile/instructions"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/builder/dockerfile/shell"
	"github.com/docker/docker/builder/fscache"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/system"
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

const (
	stepFormat = "Step %d/%d : %v"
)

// SessionGetter is object used to get access to a session by uuid
type SessionGetter interface {
	Get(ctx context.Context, uuid string) (session.Caller, error)
}

// BuildManager is shared across all Builder objects
type BuildManager struct {
	idMappings *idtools.IDMappings
	backend    builder.Backend
	pathCache  pathCache // TODO: make this persistent
	sg         SessionGetter
	fsCache    *fscache.FSCache
}

// NewBuildManager creates a BuildManager
func NewBuildManager(b builder.Backend, sg SessionGetter, fsCache *fscache.FSCache, idMappings *idtools.IDMappings) (*BuildManager, error) {
	bm := &BuildManager{
		backend:    b,
		pathCache:  &syncmap.Map{},
		sg:         sg,
		idMappings: idMappings,
		fsCache:    fsCache,
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

	os := ""
	apiPlatform := system.ParsePlatform(config.Options.Platform)
	if apiPlatform.OS != "" {
		os = apiPlatform.OS
	}
	config.Options.Platform = os

	builderOptions := builderOptions{
		Options:        config.Options,
		ProgressWriter: config.ProgressWriter,
		Backend:        bm.backend,
		PathCache:      bm.pathCache,
		IDMappings:     bm.idMappings,
	}
	return newBuilder(ctx, builderOptions).build(source, dockerfile)
}

func (bm *BuildManager) initializeClientSession(ctx context.Context, cancel func(), options *types.ImageBuildOptions) (builder.Source, error) {
	if options.SessionID == "" || bm.sg == nil {
		return nil, nil
	}
	logrus.Debug("client is session enabled")

	connectCtx, cancelCtx := context.WithTimeout(ctx, sessionConnectTimeout)
	defer cancelCtx()

	c, err := bm.sg.Get(connectCtx, options.SessionID)
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
	IDMappings     *idtools.IDMappings
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

	idMappings       *idtools.IDMappings
	disableCommit    bool
	imageSources     *imageSources
	pathCache        pathCache
	containerManager *containerManager
	imageProber      ImageProber
}

// newBuilder creates a new Dockerfile builder from an optional dockerfile and a Options.
func newBuilder(clientCtx context.Context, options builderOptions) *Builder {
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
		idMappings:       options.IDMappings,
		imageSources:     newImageSources(clientCtx, options),
		pathCache:        options.PathCache,
		imageProber:      newImageProber(options.Backend, config.CacheFrom, config.NoCache),
		containerManager: newContainerManager(options.Backend),
	}

	return b
}

// Build runs the Dockerfile builder by parsing the Dockerfile and executing
// the instructions from the file.
func (b *Builder) build(source builder.Source, dockerfile *parser.Result) (*builder.Result, error) {
	defer b.imageSources.Unmount()

	addNodesForLabelOption(dockerfile.AST, b.options.Labels)

	stages, metaArgs, err := instructions.Parse(dockerfile.AST)
	if err != nil {
		if instructions.IsUnknownInstruction(err) {
			buildsFailed.WithValues(metricsUnknownInstructionError).Inc()
		}
		return nil, errdefs.InvalidParameter(err)
	}
	if b.options.Target != "" {
		targetIx, found := instructions.HasStage(stages, b.options.Target)
		if !found {
			buildsFailed.WithValues(metricsBuildTargetNotReachableError).Inc()
			return nil, errors.Errorf("failed to reach build target %s in Dockerfile", b.options.Target)
		}
		stages = stages[:targetIx+1]
	}

	dockerfile.PrintWarnings(b.Stderr)
	dispatchState, err := b.dispatchDockerfileWithCancellation(stages, metaArgs, dockerfile.EscapeToken, source)
	if err != nil {
		return nil, err
	}
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

func processMetaArg(meta instructions.ArgCommand, shlex *shell.Lex, args *buildArgs) error {
	// shell.Lex currently only support the concatenated string format
	envs := convertMapToEnvList(args.GetAllAllowed())
	if err := meta.Expand(func(word string) (string, error) {
		return shlex.ProcessWord(word, envs)
	}); err != nil {
		return err
	}
	args.AddArg(meta.Key, meta.Value)
	args.AddMetaArg(meta.Key, meta.Value)
	return nil
}

func printCommand(out io.Writer, currentCommandIndex int, totalCommands int, cmd interface{}) int {
	fmt.Fprintf(out, stepFormat, currentCommandIndex, totalCommands, cmd)
	fmt.Fprintln(out)
	return currentCommandIndex + 1
}

func (b *Builder) dispatchDockerfileWithCancellation(parseResult []instructions.Stage, metaArgs []instructions.ArgCommand, escapeToken rune, source builder.Source) (*dispatchState, error) {
	dispatchRequest := dispatchRequest{}
	buildArgs := newBuildArgs(b.options.BuildArgs)
	totalCommands := len(metaArgs) + len(parseResult)
	currentCommandIndex := 1
	for _, stage := range parseResult {
		totalCommands += len(stage.Commands)
	}
	shlex := shell.NewLex(escapeToken)
	for _, meta := range metaArgs {
		currentCommandIndex = printCommand(b.Stdout, currentCommandIndex, totalCommands, &meta)

		err := processMetaArg(meta, shlex, buildArgs)
		if err != nil {
			return nil, err
		}
	}

	stagesResults := newStagesBuildResults()

	for _, stage := range parseResult {
		if err := stagesResults.checkStageNameAvailable(stage.Name); err != nil {
			return nil, err
		}
		dispatchRequest = newDispatchRequest(b, escapeToken, source, buildArgs, stagesResults)

		currentCommandIndex = printCommand(b.Stdout, currentCommandIndex, totalCommands, stage.SourceCode)
		if err := initializeStage(dispatchRequest, &stage); err != nil {
			return nil, err
		}
		dispatchRequest.state.updateRunConfig()
		fmt.Fprintf(b.Stdout, " ---> %s\n", stringid.TruncateID(dispatchRequest.state.imageID))
		for _, cmd := range stage.Commands {
			select {
			case <-b.clientCtx.Done():
				logrus.Debug("Builder: build cancelled!")
				fmt.Fprint(b.Stdout, "Build cancelled\n")
				buildsFailed.WithValues(metricsBuildCanceled).Inc()
				return nil, errors.New("Build cancelled")
			default:
				// Not cancelled yet, keep going...
			}

			currentCommandIndex = printCommand(b.Stdout, currentCommandIndex, totalCommands, cmd)

			if err := dispatch(dispatchRequest, cmd); err != nil {
				return nil, err
			}
			dispatchRequest.state.updateRunConfig()
			fmt.Fprintf(b.Stdout, " ---> %s\n", stringid.TruncateID(dispatchRequest.state.imageID))

		}
		if err := emitImageID(b.Aux, dispatchRequest.state); err != nil {
			return nil, err
		}
		buildArgs.MergeReferencedArgs(dispatchRequest.state.buildArgs)
		if err := commitStage(dispatchRequest.state, stagesResults); err != nil {
			return nil, err
		}
	}
	buildArgs.WarnOnUnusedBuildArgs(b.Stdout)
	return dispatchRequest.state, nil
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
func BuildFromConfig(config *container.Config, changes []string, os string) (*container.Config, error) {
	if !system.IsOSSupported(os) {
		return nil, errdefs.InvalidParameter(system.ErrNotSupportedOperatingSystem)
	}
	if len(changes) == 0 {
		return config, nil
	}

	dockerfile, err := parser.Parse(bytes.NewBufferString(strings.Join(changes, "\n")))
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	b := newBuilder(context.Background(), builderOptions{
		Options: &types.ImageBuildOptions{NoCache: true},
	})

	// ensure that the commands are valid
	for _, n := range dockerfile.AST.Children {
		if !validCommitCommands[n.Value] {
			return nil, errdefs.InvalidParameter(errors.Errorf("%s is not a valid change command", n.Value))
		}
	}

	b.Stdout = ioutil.Discard
	b.Stderr = ioutil.Discard
	b.disableCommit = true

	commands := []instructions.Command{}
	for _, n := range dockerfile.AST.Children {
		cmd, err := instructions.ParseCommand(n)
		if err != nil {
			return nil, errdefs.InvalidParameter(err)
		}
		commands = append(commands, cmd)
	}

	dispatchRequest := newDispatchRequest(b, dockerfile.EscapeToken, nil, newBuildArgs(b.options.BuildArgs), newStagesBuildResults())
	// We make mutations to the configuration, ensure we have a copy
	dispatchRequest.state.runConfig = copyRunConfig(config)
	dispatchRequest.state.imageID = config.Image
	dispatchRequest.state.operatingSystem = os
	for _, cmd := range commands {
		err := dispatch(dispatchRequest, cmd)
		if err != nil {
			return nil, errdefs.InvalidParameter(err)
		}
		dispatchRequest.state.updateRunConfig()
	}

	return dispatchRequest.state.runConfig, nil
}

func convertMapToEnvList(m map[string]string) []string {
	result := []string{}
	for k, v := range m {
		result = append(result, k+"="+v)
	}
	return result
}
