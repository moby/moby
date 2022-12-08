/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package shim

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/shutdown"
	"github.com/containerd/containerd/plugin"
	shimapi "github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/containerd/version"
	"github.com/containerd/ttrpc"
	"github.com/gogo/protobuf/proto"
	"github.com/sirupsen/logrus"
)

// Publisher for events
type Publisher interface {
	events.Publisher
	io.Closer
}

// StartOpts describes shim start configuration received from containerd
type StartOpts struct {
	ID               string // TODO(2.0): Remove ID, passed directly to start for call symmetry
	ContainerdBinary string
	Address          string
	TTRPCAddress     string
}

type StopStatus struct {
	Pid        int
	ExitStatus int
	ExitedAt   time.Time
}

// Init func for the creation of a shim server
// TODO(2.0): Remove init function
type Init func(context.Context, string, Publisher, func()) (Shim, error)

// Shim server interface
// TODO(2.0): Remove unified shim interface
type Shim interface {
	shimapi.TaskService
	Cleanup(ctx context.Context) (*shimapi.DeleteResponse, error)
	StartShim(ctx context.Context, opts StartOpts) (string, error)
}

// Manager is the interface which manages the shim process
type Manager interface {
	Name() string
	Start(ctx context.Context, id string, opts StartOpts) (string, error)
	Stop(ctx context.Context, id string) (StopStatus, error)
}

// OptsKey is the context key for the Opts value.
type OptsKey struct{}

// Opts are context options associated with the shim invocation.
type Opts struct {
	BundlePath string
	Debug      bool
}

// BinaryOpts allows the configuration of a shims binary setup
type BinaryOpts func(*Config)

// Config of shim binary options provided by shim implementations
type Config struct {
	// NoSubreaper disables setting the shim as a child subreaper
	NoSubreaper bool
	// NoReaper disables the shim binary from reaping any child process implicitly
	NoReaper bool
	// NoSetupLogger disables automatic configuration of logrus to use the shim FIFO
	NoSetupLogger bool
}

type ttrpcService interface {
	RegisterTTRPC(*ttrpc.Server) error
}

type taskService struct {
	shimapi.TaskService
}

func (t taskService) RegisterTTRPC(server *ttrpc.Server) error {
	shimapi.RegisterTaskService(server, t.TaskService)
	return nil
}

var (
	debugFlag            bool
	versionFlag          bool
	id                   string
	namespaceFlag        string
	socketFlag           string
	bundlePath           string
	addressFlag          string
	containerdBinaryFlag string
	action               string
)

const (
	ttrpcAddressEnv = "TTRPC_ADDRESS"
)

func parseFlags() {
	flag.BoolVar(&debugFlag, "debug", false, "enable debug output in logs")
	flag.BoolVar(&versionFlag, "v", false, "show the shim version and exit")
	flag.StringVar(&namespaceFlag, "namespace", "", "namespace that owns the shim")
	flag.StringVar(&id, "id", "", "id of the task")
	flag.StringVar(&socketFlag, "socket", "", "socket path to serve")
	flag.StringVar(&bundlePath, "bundle", "", "path to the bundle if not workdir")

	flag.StringVar(&addressFlag, "address", "", "grpc address back to main containerd")
	flag.StringVar(&containerdBinaryFlag, "publish-binary", "containerd", "path to publish binary (used for publishing events)")

	flag.Parse()
	action = flag.Arg(0)
}

func setRuntime() {
	debug.SetGCPercent(40)
	go func() {
		for range time.Tick(30 * time.Second) {
			debug.FreeOSMemory()
		}
	}()
	if os.Getenv("GOMAXPROCS") == "" {
		// If GOMAXPROCS hasn't been set, we default to a value of 2 to reduce
		// the number of Go stacks present in the shim.
		runtime.GOMAXPROCS(2)
	}
}

func setLogger(ctx context.Context, id string) (context.Context, error) {
	l := log.G(ctx)
	l.Logger.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: log.RFC3339NanoFixed,
		FullTimestamp:   true,
	})
	if debugFlag {
		l.Logger.SetLevel(logrus.DebugLevel)
	}
	f, err := openLog(ctx, id)
	if err != nil { //nolint:staticcheck // Ignore SA4023 as some platforms always return error
		return ctx, err
	}
	l.Logger.SetOutput(f)
	return log.WithLogger(ctx, l), nil
}

// Run initializes and runs a shim server
// TODO(2.0): Remove function
func Run(name string, initFunc Init, opts ...BinaryOpts) {
	var config Config
	for _, o := range opts {
		o(&config)
	}

	ctx := context.Background()
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("runtime", name))

	if err := run(ctx, nil, initFunc, name, config); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s", name, err)
		os.Exit(1)
	}
}

// TODO(2.0): Remove this type
type shimToManager struct {
	shim Shim
	name string
}

func (stm shimToManager) Name() string {
	return stm.name
}

func (stm shimToManager) Start(ctx context.Context, id string, opts StartOpts) (string, error) {
	opts.ID = id
	return stm.shim.StartShim(ctx, opts)
}

func (stm shimToManager) Stop(ctx context.Context, id string) (StopStatus, error) {
	// shim must already have id
	dr, err := stm.shim.Cleanup(ctx)
	if err != nil {
		return StopStatus{}, err
	}
	return StopStatus{
		Pid:        int(dr.Pid),
		ExitStatus: int(dr.ExitStatus),
		ExitedAt:   dr.ExitedAt,
	}, nil
}

// RunManager initialzes and runs a shim server
// TODO(2.0): Rename to Run
func RunManager(ctx context.Context, manager Manager, opts ...BinaryOpts) {
	var config Config
	for _, o := range opts {
		o(&config)
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithField("runtime", manager.Name()))

	if err := run(ctx, manager, nil, "", config); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s", manager.Name(), err)
		os.Exit(1)
	}
}

func run(ctx context.Context, manager Manager, initFunc Init, name string, config Config) error {
	parseFlags()
	if versionFlag {
		fmt.Printf("%s:\n", os.Args[0])
		fmt.Println("  Version: ", version.Version)
		fmt.Println("  Revision:", version.Revision)
		fmt.Println("  Go version:", version.GoVersion)
		fmt.Println("")
		return nil
	}

	if namespaceFlag == "" {
		return fmt.Errorf("shim namespace cannot be empty")
	}

	setRuntime()

	signals, err := setupSignals(config)
	if err != nil { //nolint:staticcheck // Ignore SA4023 as some platforms always return error
		return err
	}

	if !config.NoSubreaper {
		if err := subreaper(); err != nil { //nolint:staticcheck // Ignore SA4023 as some platforms always return error
			return err
		}
	}

	ttrpcAddress := os.Getenv(ttrpcAddressEnv)
	publisher, err := NewPublisher(ttrpcAddress)
	if err != nil {
		return err
	}
	defer publisher.Close()

	ctx = namespaces.WithNamespace(ctx, namespaceFlag)
	ctx = context.WithValue(ctx, OptsKey{}, Opts{BundlePath: bundlePath, Debug: debugFlag})
	ctx, sd := shutdown.WithShutdown(ctx)
	defer sd.Shutdown()

	if manager == nil {
		service, err := initFunc(ctx, id, publisher, sd.Shutdown)
		if err != nil {
			return err
		}
		plugin.Register(&plugin.Registration{
			Type: plugin.TTRPCPlugin,
			ID:   "task",
			Requires: []plugin.Type{
				plugin.EventPlugin,
			},
			InitFn: func(ic *plugin.InitContext) (interface{}, error) {
				return taskService{service}, nil
			},
		})
		manager = shimToManager{
			shim: service,
			name: name,
		}
	}

	// Handle explicit actions
	switch action {
	case "delete":
		logger := log.G(ctx).WithFields(logrus.Fields{
			"pid":       os.Getpid(),
			"namespace": namespaceFlag,
		})
		go reap(ctx, logger, signals)
		ss, err := manager.Stop(ctx, id)
		if err != nil {
			return err
		}
		data, err := proto.Marshal(&shimapi.DeleteResponse{
			Pid:        uint32(ss.Pid),
			ExitStatus: uint32(ss.ExitStatus),
			ExitedAt:   ss.ExitedAt,
		})
		if err != nil {
			return err
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return err
		}
		return nil
	case "start":
		opts := StartOpts{
			ContainerdBinary: containerdBinaryFlag,
			Address:          addressFlag,
			TTRPCAddress:     ttrpcAddress,
		}

		address, err := manager.Start(ctx, id, opts)
		if err != nil {
			return err
		}
		if _, err := os.Stdout.WriteString(address); err != nil {
			return err
		}
		return nil
	}

	if !config.NoSetupLogger {
		ctx, err = setLogger(ctx, id)
		if err != nil {
			return err
		}
	}

	plugin.Register(&plugin.Registration{
		Type: plugin.InternalPlugin,
		ID:   "shutdown",
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			return sd, nil
		},
	})

	// Register event plugin
	plugin.Register(&plugin.Registration{
		Type: plugin.EventPlugin,
		ID:   "publisher",
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			return publisher, nil
		},
	})

	var (
		initialized   = plugin.NewPluginSet()
		ttrpcServices = []ttrpcService{}
	)
	plugins := plugin.Graph(func(*plugin.Registration) bool { return false })
	for _, p := range plugins {
		id := p.URI()
		log.G(ctx).WithField("type", p.Type).Infof("loading plugin %q...", id)

		initContext := plugin.NewContext(
			ctx,
			p,
			initialized,
			// NOTE: Root is empty since the shim does not support persistent storage,
			// shim plugins should make use state directory for writing files to disk.
			// The state directory will be destroyed when the shim if cleaned up or
			// on reboot
			"",
			bundlePath,
		)
		initContext.Address = addressFlag
		initContext.TTRPCAddress = ttrpcAddress

		// load the plugin specific configuration if it is provided
		//TODO: Read configuration passed into shim, or from state directory?
		//if p.Config != nil {
		//	pc, err := config.Decode(p)
		//	if err != nil {
		//		return nil, err
		//	}
		//	initContext.Config = pc
		//}

		result := p.Init(initContext)
		if err := initialized.Add(result); err != nil {
			return fmt.Errorf("could not add plugin result to plugin set: %w", err)
		}

		instance, err := result.Instance()
		if err != nil {
			if plugin.IsSkipPlugin(err) {
				log.G(ctx).WithError(err).WithField("type", p.Type).Infof("skip loading plugin %q...", id)
			} else {
				log.G(ctx).WithError(err).Warnf("failed to load plugin %s", id)
			}
			continue
		}

		if src, ok := instance.(ttrpcService); ok {
			logrus.WithField("id", id).Debug("registering ttrpc service")
			ttrpcServices = append(ttrpcServices, src)
		}
	}

	server, err := newServer()
	if err != nil { //nolint:staticcheck // Ignore SA4023 as some platforms always return error
		return fmt.Errorf("failed creating server: %w", err)
	}

	for _, srv := range ttrpcServices {
		if err := srv.RegisterTTRPC(server); err != nil {
			return fmt.Errorf("failed to register service: %w", err)
		}
	}

	if err := serve(ctx, server, signals, sd.Shutdown); err != nil { //nolint:staticcheck // Ignore SA4023 as some platforms always return error
		if err != shutdown.ErrShutdown {
			return err
		}
	}

	// NOTE: If the shim server is down(like oom killer), the address
	// socket might be leaking.
	if address, err := ReadAddress("address"); err == nil {
		_ = RemoveSocket(address)
	}

	select {
	case <-publisher.Done():
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("publisher not closed")
	}
}

// serve serves the ttrpc API over a unix socket in the current working directory
// and blocks until the context is canceled
func serve(ctx context.Context, server *ttrpc.Server, signals chan os.Signal, shutdown func()) error {
	dump := make(chan os.Signal, 32)
	setupDumpStacks(dump)

	path, err := os.Getwd()
	if err != nil {
		return err
	}

	l, err := serveListener(socketFlag)
	if err != nil { //nolint:staticcheck // Ignore SA4023 as some platforms always return error
		return err
	}
	go func() {
		defer l.Close()
		if err := server.Serve(ctx, l); err != nil &&
			!strings.Contains(err.Error(), "use of closed network connection") {
			log.G(ctx).WithError(err).Fatal("containerd-shim: ttrpc server failure")
		}
	}()
	logger := log.G(ctx).WithFields(logrus.Fields{
		"pid":       os.Getpid(),
		"path":      path,
		"namespace": namespaceFlag,
	})
	go func() {
		for range dump {
			dumpStacks(logger)
		}
	}()

	go handleExitSignals(ctx, logger, shutdown)
	return reap(ctx, logger, signals)
}

func dumpStacks(logger *logrus.Entry) {
	var (
		buf       []byte
		stackSize int
	)
	bufferLen := 16384
	for stackSize == len(buf) {
		buf = make([]byte, bufferLen)
		stackSize = runtime.Stack(buf, true)
		bufferLen *= 2
	}
	buf = buf[:stackSize]
	logger.Infof("=== BEGIN goroutine stack dump ===\n%s\n=== END goroutine stack dump ===", buf)
}
