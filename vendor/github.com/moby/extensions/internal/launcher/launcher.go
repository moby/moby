package launcher

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/moby/extensions"
	"github.com/moby/extensions/sdk"
	"github.com/moby/extensions/sdk/sdkapi"
	sdkapipb "github.com/moby/extensions/sdk/sdkapi/protogen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Launcher starts and describes out-of-process extensions.
type Launcher struct {
	RuntimeDir      string
	ReadyTimeout    time.Duration
	ShutdownTimeout time.Duration
	// ExtensionConfig holds configuration keyed by extension id. The selected
	// entry is delivered over the startup handshake.
	ExtensionConfig map[extensions.ExtensionID]extensions.Config
	// CallbackEndpoint is the socket where the host serves dependencies.
	CallbackEndpoint string
}

// Launched is a started out-of-process extension and its connection.
type Launched struct {
	ID           extensions.ExtensionID
	Dependencies []extensions.Dependency
	Conflicts    []extensions.ExtensionID
	Points       []LaunchedPoint
	// ProviderServices are the fully-qualified gRPC service names the extension
	// serves for each provider point on its per-extension socket. The host, not
	// the SDK, decides which point's services are also published on the daemon API
	// socket.
	ProviderServices map[extensions.PointID][]string
	Conn             grpc.ClientConnInterface
	shutdown         *processShutdown
}

// LaunchedPoint is one point an extension declared it provides.
type LaunchedPoint struct {
	ID extensions.PointID
}

// Close stops the extension process and closes the connection.
func (l *Launched) Close(ctx context.Context) error {
	return l.shutdown.Close(ctx)
}

// Initialize runs the extension's Init over the Initialize RPC.
func (l *Launched) Initialize(ctx context.Context) error {
	_, err := sdkapipb.NewClient(l.Conn).Initialize(ctx, &sdkapi.InitializeRequest{})
	return err
}

// Launch starts the extension binary bin, performs the stdio handshake, and
// describes it. The executable's file name (minus any .exe on Windows) is its
// extension id, which the launched extension must declare to match.
func (l Launcher) Launch(ctx context.Context, bin string) (*Launched, error) {
	readyTimeout := l.ReadyTimeout
	if readyTimeout == 0 {
		readyTimeout = 5 * time.Second
	}
	shutdownTimeout := l.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 5 * time.Second
	}
	if l.RuntimeDir == "" {
		return nil, errors.New("extension runtime dir is required")
	}
	if err := os.MkdirAll(l.RuntimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("create extension runtime dir: %w", err)
	}
	name := extensionName(filepath.Base(bin))
	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf("extension %q: %w", name, err)
	}
	endpoint := extensionSocketPath(l.RuntimeDir, name)
	if err := os.Remove(endpoint); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale extension socket: %w", err)
	}

	cmd := exec.CommandContext(ctx, bin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open extension stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open extension stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("open extension stderr: %w", err)
	}
	go logOutput(ctx, name, stderr)
	lifetime, err := startProcess(cmd)
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("start extension %q: %w", name, err)
	}
	// Reap the child immediately. Later shutdown reads this channel because Wait
	// may only be called once.
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	stop := func() {
		_ = stopProcess(context.Background(), cmd, wait, shutdownTimeout)
		_ = lifetime.Close()
	}
	startup := sdk.StartupConfig{
		Endpoint:         endpoint,
		ProtocolVersion:  sdk.ProtocolVersion,
		Config:           l.ExtensionConfig[extensions.ExtensionID(name)],
		CallbackEndpoint: l.CallbackEndpoint,
	}
	if err := json.NewEncoder(stdin).Encode(startup); err != nil {
		stop()
		return nil, fmt.Errorf("write startup config for extension %q: %w", name, err)
	}
	_ = stdin.Close()

	readyCtx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	stdoutBuf := bufio.NewReader(stdout)
	if err := waitReady(readyCtx, stdout, stdoutBuf); err != nil {
		stop()
		return nil, fmt.Errorf("wait for extension %q readiness: %w", name, err)
	}
	// Keep draining stdout after the readiness ack so the pipe cannot fill and
	// block the extension.
	go logOutput(ctx, name, stdoutBuf)

	conn, err := dial(endpoint)
	if err != nil {
		stop()
		return nil, fmt.Errorf("connect to extension %q: %w", name, err)
	}
	resp, err := sdkapipb.NewClient(conn).Describe(ctx, &sdkapi.DescribeRequest{})
	if err != nil {
		_ = conn.Close()
		stop()
		return nil, fmt.Errorf("describe extension %q: %w", name, err)
	}
	decl := resp.Declaration
	if decl == nil || decl.ID == "" {
		_ = conn.Close()
		stop()
		return nil, fmt.Errorf("extension %q described no extension", name)
	}
	if decl.ID != name {
		_ = conn.Close()
		stop()
		return nil, fmt.Errorf("extension %q declared id %q, which must match its file name", name, decl.ID)
	}
	launched := &Launched{
		ID:               extensions.ExtensionID(decl.ID),
		Dependencies:     declaredDependencies(decl.Dependencies),
		Conflicts:        declaredConflicts(decl.Conflicts),
		ProviderServices: declaredServices(decl.ProviderServices),
		Conn:             conn,
		shutdown:         &processShutdown{conn: conn, cmd: cmd, wait: wait, timeout: shutdownTimeout, lifetime: lifetime},
	}
	for _, p := range decl.Providers {
		launched.Points = append(launched.Points, LaunchedPoint{
			ID: extensions.PointID(p.ID),
		})
	}
	// Require service inventories to belong to declared provider points. This
	// keeps socket exposure behind the point's explicit opt-in.
	if err := validateDeclaredServices(name, launched); err != nil {
		_ = conn.Close()
		stop()
		return nil, err
	}
	return launched, nil
}

// extensionSocketPath keeps the private AF_UNIX socket name bounded regardless
// of extension ID length. Windows and Unix both impose short sockaddr limits.
func extensionSocketPath(runtimeDir, name string) string {
	sum := sha256.Sum256([]byte(name))
	socketName := base64.RawURLEncoding.EncodeToString(sum[:16]) + ".sock"
	return filepath.Join(runtimeDir, socketName)
}

// dial returns a lazy connection to the extension's Unix socket.
func dial(endpoint string) (*grpc.ClientConn, error) {
	return grpc.NewClient("unix:"+endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", endpoint)
		}),
	)
}

func waitReady(ctx context.Context, stdout io.Closer, r *bufio.Reader) error {
	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		line, err := r.ReadString('\n')
		done <- result{line: line, err: err}
	}()
	select {
	case <-ctx.Done():
		// Closing stdout unblocks ReadString so the goroutine does not outlive this
		// call.
		_ = stdout.Close()
		return ctx.Err()
	case res := <-done:
		if res.err != nil {
			return res.err
		}
		if res.line != sdk.ReadinessAck {
			return fmt.Errorf("unexpected readiness ack %q", res.line)
		}
		return nil
	}
}
