//go:build (linux || windows) && !no_embedded_containerd

package embedded

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/containerd/containerd/v2/cmd/containerd/server"
	srvconfig "github.com/containerd/containerd/v2/cmd/containerd/server/config"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/containerd/v2/version"
	"github.com/containerd/log"
	"github.com/prometheus/client_golang/prometheus"

	// Cross-platform plugin registrations. Each blank import registers one or
	// more containerd plugins that server.New then loads. Platform-specific
	// snapshotters and differs are registered in embedded_linux.go /
	// embedded_windows.go.
	//
	// This is a trimmed subset of containerd's cmd/containerd/builtins: dockerd
	// does not need CRI, sandbox, streaming, transfer, NRI, or the restart
	// monitor.
	_ "github.com/containerd/containerd/v2/core/runtime/v2"
	_ "github.com/containerd/containerd/v2/plugins/content/local/plugin"
	_ "github.com/containerd/containerd/v2/plugins/events"
	_ "github.com/containerd/containerd/v2/plugins/gc"
	_ "github.com/containerd/containerd/v2/plugins/leases"
	_ "github.com/containerd/containerd/v2/plugins/metadata"
	_ "github.com/containerd/containerd/v2/plugins/mount"
	_ "github.com/containerd/containerd/v2/plugins/services/containers"
	_ "github.com/containerd/containerd/v2/plugins/services/content"
	_ "github.com/containerd/containerd/v2/plugins/services/diff"
	_ "github.com/containerd/containerd/v2/plugins/services/events"
	_ "github.com/containerd/containerd/v2/plugins/services/healthcheck"
	_ "github.com/containerd/containerd/v2/plugins/services/images"
	_ "github.com/containerd/containerd/v2/plugins/services/introspection"
	_ "github.com/containerd/containerd/v2/plugins/services/leases"
	_ "github.com/containerd/containerd/v2/plugins/services/namespaces"
	_ "github.com/containerd/containerd/v2/plugins/services/snapshots"
	_ "github.com/containerd/containerd/v2/plugins/services/tasks"
	_ "github.com/containerd/containerd/v2/plugins/services/version"
	_ "github.com/containerd/containerd/v2/plugins/services/warning"
)

// Start initializes and runs the embedded containerd server, using the daemon
// subdirectory under rootDir for persistent state (bolt DB, content store) and
// the daemon subdirectory under stateDir for runtime state. See the package doc
// for the transport layout. Plugins self-register via the blank imports above
// and in the platform-specific files.
func Start(ctx context.Context, rootDir, stateDir string, opts ...DaemonOpt) (Daemon, error) {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}

	setContainerdVersion()

	address := defaultAddress(stateDir)
	srvCfg := buildSrvConfig(cfg, rootDir, stateDir, address)

	// Create the root and state directories with the permissions containerd
	// expects (e.g. the state dir at 0o711 for userns-remapped containers),
	// matching what containerd's own command does before server.New.
	if err := server.CreateTopLevelDirectories(srvCfg); err != nil {
		return nil, err
	}

	log.G(ctx).WithField("address", address).Info("starting embedded containerd server")
	srv, err := newServer(ctx, srvCfg)
	if err != nil {
		return nil, err
	}

	socket, err := listen(srvCfg.GRPC.Address)
	if err != nil {
		srv.Stop()
		return nil, err
	}
	// Shims (separate processes) publish task events back to containerd over
	// ttrpc, so it must be a real socket. The runtime plugin hands its address
	// (config.TTRPC.Address) to each shim.
	ttrpcL, err := listen(srvCfg.TTRPC.Address)
	if err != nil {
		srv.Stop()
		_ = socket.Close()
		return nil, err
	}
	inMemory := newInMemoryListener()

	e := &embeddedDaemon{srv: srv, address: address, inMemory: inMemory, ttrpcL: ttrpcL, stopCh: make(chan struct{})}

	var wg sync.WaitGroup
	serve := func(name string, l net.Listener, fn func(net.Listener) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Serve returns an error once the server is stopped, which is
			// expected during Shutdown, so only log unexpected failures.
			if err := fn(l); err != nil && !e.stopping.Load() {
				log.G(ctx).WithError(err).Errorf("embedded containerd %s server exited", name)
			}
		}()
	}
	serve("gRPC socket", socket, srv.ServeGRPC)
	serve("gRPC in-memory", inMemory, srv.ServeGRPC)
	serve("ttrpc", ttrpcL, srv.ServeTTRPC)
	go func() {
		wg.Wait()
		close(e.stopCh)
	}()

	// Tie the server to the daemon context: shut down when it is cancelled,
	// leaving running shims untouched. WithoutCancel so Shutdown can still wait
	// for the server to stop after ctx is already done.
	go func() {
		<-ctx.Done()
		if err := e.Shutdown(context.WithoutCancel(ctx)); err != nil {
			log.G(ctx).WithError(err).Error("failed to shut down embedded containerd")
		}
	}()

	return e, nil
}

// setContainerdVersion makes the embedded server report the vendored containerd
// version in "docker info" instead of the source default ("2.2.4+unknown").
//
// The version comes from the build's module info. The git commit is not
// recorded there, so only the version is set.
func setContainerdVersion() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, dep := range bi.Deps {
		if dep.Path == "github.com/containerd/containerd/v2" && dep.Version != "" {
			version.Version = dep.Version
			break
		}
	}
}

// newServer initializes the embedded containerd server.
//
// containerd's server.New adds gRPC metrics to the global Prometheus registry
// with MustRegister, which panics if they are already there. BuildKit adds the
// same metrics in dockerd, so point containerd's registration at a temporary
// registry just for this call. The embedded server's gRPC metrics do not need
// to be exposed anyway.
//
// This is safe because Start runs early in startup, before anything else
// registers metrics.
func newServer(ctx context.Context, cfg *srvconfig.Config) (*server.Server, error) {
	saved := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	defer func() { prometheus.DefaultRegisterer = saved }()
	return server.New(ctx, cfg)
}

func buildSrvConfig(cfg *Config, rootDir, stateDir, address string) *srvconfig.Config {
	srvCfg := &srvconfig.Config{
		Version: 2,
		Root:    filepath.Join(rootDir, "daemon"),
		State:   filepath.Join(stateDir, "daemon"),
		GRPC: srvconfig.GRPCConfig{
			Address:        address,
			MaxRecvMsgSize: defaults.DefaultMaxRecvMsgSize,
			MaxSendMsgSize: defaults.DefaultMaxSendMsgSize,
		},
		TTRPC: srvconfig.TTRPCConfig{
			Address: address + ".ttrpc",
		},
		DisabledPlugins: cfg.DisabledPlugins,
	}
	if cfg.LogLevel != "" {
		srvCfg.Debug.Level = cfg.LogLevel
	}
	return srvCfg
}

type embeddedDaemon struct {
	srv      *server.Server
	address  string
	inMemory *inMemoryListener
	ttrpcL   net.Listener
	stopCh   chan struct{}
	stopping atomic.Bool
}

func (e *embeddedDaemon) Address() string {
	return e.address
}

// Dial returns one end of an in-memory pipe connected to the gRPC server. The
// addr argument is ignored, and only present to satisfy grpc.WithContextDialer.
func (e *embeddedDaemon) Dial(ctx context.Context, _ string) (net.Conn, error) {
	serverConn, clientConn := net.Pipe()
	select {
	case e.inMemory.ch <- serverConn:
		return clientConn, nil
	case <-ctx.Done():
		_ = serverConn.Close()
		_ = clientConn.Close()
		return nil, ctx.Err()
	case <-e.inMemory.done:
		_ = serverConn.Close()
		_ = clientConn.Close()
		return nil, errors.New("embedded containerd server is stopped")
	}
}

func (e *embeddedDaemon) WaitTimeout(d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return errors.New("timeout waiting for embedded containerd to stop")
	case <-e.stopCh:
		return nil
	}
}

func (e *embeddedDaemon) Shutdown(ctx context.Context) error {
	e.stopping.Store(true)
	// Stop closes the gRPC server and its listeners (socket + pipe). The ttrpc
	// server is independent, so close its listener to make ServeTTRPC return.
	e.srv.Stop()
	e.inMemory.Close()
	_ = e.ttrpcL.Close()
	select {
	case <-e.stopCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// inMemoryListener is a net.Listener whose connections are supplied by
// Dial rather than accepted from the kernel.
type inMemoryListener struct {
	ch   chan net.Conn
	done chan struct{}
	once sync.Once
}

func newInMemoryListener() *inMemoryListener {
	return &inMemoryListener{
		ch:   make(chan net.Conn, 16),
		done: make(chan struct{}),
	}
}

func (l *inMemoryListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.ch:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *inMemoryListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *inMemoryListener) Addr() net.Addr {
	return &net.UnixAddr{Name: "embedded-containerd", Net: "inmem"}
}
