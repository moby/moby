package dockerfile

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/client/session"
	"github.com/docker/docker/client/session/auth"
	"github.com/docker/docker/client/session/echo"
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

// SessionGetter is object used to get access to a session by uuid
type SessionGetter interface {
	GetSession(ctx context.Context, uuid string) (context.Context, session.Caller, error)
}

// BuildManager is shared across all Builder objects
type BuildManager struct {
	backend   builder.Backend
	pathCache pathCache // TODO: make this persistent
	sg        SessionGetter
}

// NewBuildManager creates a BuildManager
func NewBuildManager(b builder.Backend, sg SessionGetter) (*BuildManager, error) {
	bm := &BuildManager{
		backend:   b,
		pathCache: &syncmap.Map{},
		sg:        sg,
	}
	return bm, nil
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if config.Options.SessionID != "" && bm.sg != nil {
		logrus.Debug("client is session enabled")
		sessionCtx, c, err := bm.sg.GetSession(ctx, config.Options.SessionID)
		if err != nil {
			return nil, err
		}
		defer c.Close()
		go func() {
			<-sessionCtx.Done()
			cancel()
		}()
	}

	builderOptions := builderOptions{
		Options:        config.Options,
		ProgressWriter: config.ProgressWriter,
		Backend:        bm.backend,
		PathCache:      bm.pathCache,
	}

	if bm.sg != nil && config.Options.SessionID != "" {
		_, c, _ := bm.sg.GetSession(ctx, config.Options.SessionID)
		if c != nil {
			if p, ok := auth.TryGetAuthConfigProviderClient(c); ok {
				builderOptions.authProvider = p
			}
			echo1Messages := make(chan string)
			echo1Callbacks := make(chan string)
			echo2Messages := make(chan string)
			echo2Callbacks := make(chan string)
			if ok, _ := echo.TrySetupEchoClient(ctx, c, "echo1", echo1Messages, echo1Callbacks); ok {
				go func() {
					echo1Messages <- "Message for echo1 : 1"
					echo1Messages <- "Message for echo1 : 2"
					echo1Messages <- "Message for echo1 : 3"
					close(echo1Messages)
				}()
				go func() {
					for m := range echo1Callbacks {
						logrus.Debugf("Received from echo1: %s", m)
					}
					logrus.Debugf("echo1 closed")
				}()
			}
			if ok, _ := echo.TrySetupEchoClient(ctx, c, "echo2", echo2Messages, echo2Callbacks); ok {
				go func() {
					echo2Messages <- "Message for echo2 : 1"
					echo2Messages <- "Message for echo2 : 2"
					echo2Messages <- "Message for echo2 : 3"
					close(echo2Messages)
				}()
				go func() {
					for m := range echo2Callbacks {
						logrus.Debugf("Received from echo2: %s", m)
					}
					logrus.Debugf("echo2 closed")
				}()
			}
		}
	}
	if builderOptions.authProvider == nil {
		builderOptions.authProvider = &staticAuthConfigProvider{auths: config.Options.AuthConfigs}
	}

	return newBuilder(ctx, builderOptions).build(source, dockerfile)
}

// builderOptions are the dependencies required by the builder
type builderOptions struct {
	Options        *types.ImageBuildOptions
	Backend        builder.Backend
	ProgressWriter backend.ProgressWriter
	PathCache      pathCache
	authProvider   auth.AuthConfigProvider
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

	tmpContainers map[string]struct{}
	imageContexts *imageContexts // helper for storing contexts from builds
	disableCommit bool
	cacheBusted   bool
	buildArgs     *buildArgs
	imageCache    builder.ImageCache
	authProvider  auth.AuthConfigProvider
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
		tmpContainers: map[string]struct{}{},
		buildArgs:     newBuildArgs(config.BuildArgs),
		authProvider:  options.authProvider,
	}

	b.imageContexts = &imageContexts{b: b, cache: options.PathCache}
	return b
}

func (b *Builder) resetImageCache() {
	if icb, ok := b.docker.(builder.ImageCacheBuilder); ok {
		b.imageCache = icb.MakeImageCache(b.options.CacheFrom)
	}
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

	dispatchState, err := b.dispatchDockerfileWithCancellation(dockerfile)
	if err != nil {
		return nil, err
	}

	if b.options.Target != "" && !dispatchState.isCurrentStage(b.options.Target) {
		return nil, errors.Errorf("failed to reach build target %s in Dockerfile", b.options.Target)
	}

	b.warnOnUnusedBuildArgs()

	if dispatchState.imageID == "" {
		return nil, errors.New("No image was generated. Is your Dockerfile empty?")
	}
	return &builder.Result{ImageID: dispatchState.imageID, FromImage: dispatchState.baseImage}, nil
}

func (b *Builder) dispatchDockerfileWithCancellation(dockerfile *parser.Result) (*dispatchState, error) {
	shlex := NewShellLex(dockerfile.EscapeToken)
	state := newDispatchState()
	total := len(dockerfile.AST.Children)
	var err error
	for i, n := range dockerfile.AST.Children {
		select {
		case <-b.clientCtx.Done():
			logrus.Debug("Builder: build cancelled!")
			fmt.Fprint(b.Stdout, "Build cancelled")
			return nil, errors.New("Build cancelled")
		default:
			// Not cancelled yet, keep going...
		}

		if n.Value == command.From && state.isCurrentStage(b.options.Target) {
			break
		}

		opts := dispatchOptions{
			state:   state,
			stepMsg: formatStep(i, total),
			node:    n,
			shlex:   shlex,
		}
		if state, err = b.dispatch(opts); err != nil {
			if b.options.ForceRemove {
				b.clearTmp()
			}
			return nil, err
		}

		fmt.Fprintf(b.Stdout, " ---> %s\n", stringid.TruncateID(state.imageID))
		if b.options.Remove {
			b.clearTmp()
		}
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

// check if there are any leftover build-args that were passed but not
// consumed during build. Print a warning, if there are any.
func (b *Builder) warnOnUnusedBuildArgs() {
	leftoverArgs := b.buildArgs.UnreferencedOptionArgs()
	if len(leftoverArgs) > 0 {
		fmt.Fprintf(b.Stderr, "[Warning] One or more build-args %v were not consumed\n", leftoverArgs)
	}
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

	b := newBuilder(context.Background(), builderOptions{authProvider: &nilAuthConfigProvider{}})

	dockerfile, err := parser.Parse(bytes.NewBufferString(strings.Join(changes, "\n")))
	if err != nil {
		return nil, err
	}

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
	return dispatchFromDockerfile(b, dockerfile, dispatchState)
}

func checkDispatchDockerfile(dockerfile *parser.Node) error {
	for _, n := range dockerfile.Children {
		if err := checkDispatch(n); err != nil {
			return errors.Wrapf(err, "Dockerfile parse error line %d", n.StartLine)
		}
	}
	return nil
}

func dispatchFromDockerfile(b *Builder, result *parser.Result, dispatchState *dispatchState) (*container.Config, error) {
	shlex := NewShellLex(result.EscapeToken)
	ast := result.AST
	total := len(ast.Children)

	for i, n := range ast.Children {
		opts := dispatchOptions{
			state:   dispatchState,
			stepMsg: formatStep(i, total),
			node:    n,
			shlex:   shlex,
		}
		if _, err := b.dispatch(opts); err != nil {
			return nil, err
		}
	}
	return dispatchState.runConfig, nil
}

type tmpCacheBackend struct {
	root string
}

func (tcb *tmpCacheBackend) Get(id string) (string, error) {
	d := filepath.Join(tcb.root, id)
	if err := os.MkdirAll(d, 0700); err != nil {
		return "", errors.Wrapf(err, "failed to create tmp dir for %s", d)
	}
	return d, nil
}
func (tcb *tmpCacheBackend) Remove(id string) error {
	return os.RemoveAll(filepath.Join(tcb.root, id))
}
