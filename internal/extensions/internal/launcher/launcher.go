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
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/internal/extensions"
	"github.com/moby/moby/v2/internal/extensions/sdk"
	"github.com/moby/moby/v2/internal/extensions/sdk/sdkapi"
	sdkapipb "github.com/moby/moby/v2/internal/extensions/sdk/sdkapi/protogen"
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

// Binaries lists executable extensions directly under dir. Each is named after
// its file, without .exe on Windows. A missing directory yields no binaries.
//
// Discovery is a root-code-execution boundary. World-writable entries, entries
// owned by an untrusted user, and files without valid extension ids are skipped.
// Other trust decisions, including group policy and symlinks, belong to the
// operator.
func Binaries(ctx context.Context, dir string) ([]string, error) {
	dirInfo, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat extension dir %q: %w", dir, err)
	}
	if worldWritable(dirInfo) {
		log.G(ctx).Warnf("extensions: ignoring world-writable extension directory %q", dir)
		return nil, nil
	}
	if uid, untrusted := untrustedOwner(dirInfo); untrusted {
		log.G(ctx).Warnf("extensions: ignoring extension directory %q owned by untrusted uid %d", dir, uid)
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read extension dir %q: %w", dir, err)
	}
	var bins []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat extension %q: %w", filepath.Join(dir, e.Name()), err)
		}
		if !isExecutable(info) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		// The file name is the extension id. Validate it before launching so a
		// shared directory cannot execute arbitrary helper binaries.
		name := extensionName(e.Name())
		if err := extensions.ValidateExtensionID(extensions.ExtensionID(name)); err != nil {
			log.G(ctx).WithError(err).Warnf("extensions: skipping %q: not a valid extension binary name", path)
			continue
		}
		if worldWritable(info) {
			log.G(ctx).Warnf("extensions: refusing to run world-writable extension binary %q", path)
			continue
		}
		if uid, untrusted := untrustedOwner(info); untrusted {
			log.G(ctx).Warnf("extensions: refusing to run extension binary %q owned by untrusted uid %d", path, uid)
			continue
		}
		bins = append(bins, path)
	}
	return bins, nil
}

// untrustedOwner reports whether info is owned by a uid that is neither the
// superuser (0) nor the daemon's own effective user, returning that uid when so.
// A binary or directory owned by any other user could be rewritten by them and
// then executed as the daemon, so it is not trusted. This complements the
// world-writable check: that catches a file anyone can rewrite, this catches one
// a specific untrusted owner can. Ownership is not determinable on every platform
// (notably Windows, where access is governed by ACLs the mode does not reflect);
// there it is not enforced, and broader owner and group policy remains the
// operator's, per the security model in the design docs.
func untrustedOwner(info fs.FileInfo) (int, bool) {
	uid, ok := fileUID(info)
	if !ok {
		return 0, false
	}
	if uid == 0 || uid == os.Geteuid() {
		return 0, false
	}
	return uid, true
}

func isExecutable(info fs.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Ext(info.Name()), ".exe")
	}
	return info.Mode().Perm()&0o111 != 0
}

func extensionName(file string) string {
	if runtime.GOOS == "windows" {
		ext := filepath.Ext(file)
		if strings.EqualFold(ext, ".exe") {
			return strings.TrimSuffix(file, ext)
		}
	}
	return strings.TrimSuffix(file, ".exe")
}

// worldWritable reports whether info is writable by others (the o+w bit). A
// world-writable binary or directory on the daemon's exec path lets any local
// user run code as the daemon, so it is not trusted. The bit is only meaningful
// on Unix; on Windows access is governed by ACLs the mode does not reflect, so
// this check does not apply there.
func worldWritable(info fs.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return info.Mode().Perm()&0o002 != 0
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

// validateDeclaredServices rejects services listed under a point the extension
// did not declare it provides.
func validateDeclaredServices(name string, launched *Launched) error {
	declared := make(map[extensions.PointID]bool, len(launched.Points))
	for _, p := range launched.Points {
		declared[p.ID] = true
	}
	for point := range launched.ProviderServices {
		if !declared[point] {
			return fmt.Errorf("extension %q serves services for point %q without declaring it", name, point)
		}
	}
	return nil
}

// declaredServices converts the service inventory reported by the SDK:
// service names grouped by the provider point whose ServerPoint registered them.
func declaredServices(in []sdkapi.ProviderServices) map[extensions.PointID][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[extensions.PointID][]string, len(in))
	for _, ps := range in {
		point := extensions.PointID(ps.Point)
		if point == "" || len(ps.Services) == 0 {
			continue
		}
		out[point] = append(out[point], ps.Services...)
	}
	return out
}

type processLifetime interface {
	Close() error
}

type processShutdown struct {
	conn     *grpc.ClientConn
	cmd      *exec.Cmd
	wait     <-chan error
	timeout  time.Duration
	lifetime processLifetime
	once     sync.Once
	err      error
}

// Close stops the extension once. The host and broker may both call it during
// failure cleanup, so repeated calls are no-ops.
func (s *processShutdown) Close(ctx context.Context) error {
	s.once.Do(func() {
		s.err = errors.Join(
			s.conn.Close(),
			stopProcess(ctx, s.cmd, s.wait, s.timeout),
			s.lifetime.Close(),
		)
	})
	return s.err
}

// stopErr ignores the exit status of a process that was deliberately stopped.
func stopErr(err error) error {
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil
	}
	return err
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

func stopProcess(ctx context.Context, cmd *exec.Cmd, done <-chan error, timeout time.Duration) error {
	if cmd.Process == nil {
		return nil
	}

	// Ask the extension to stop. If signaling is unsupported, fall back to Kill.
	if err := cmd.Process.Signal(shutdownSignal()); err != nil && !errors.Is(err, os.ErrProcessDone) {
		// os.ErrProcessDone means it already exited, which is the stop we
		// wanted; any other Kill error means we failed to stop it, so report it.
		if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return fmt.Errorf("kill extension after failed signal %v: %w", err, killErr)
		}
		<-done
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case err := <-done:
		return stopErr(err)
	case <-shutdownCtx.Done():
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		<-done
		return nil
	}
}

// logOutput drains extension output at info level. A bufio.Reader avoids the
// scanner token limit, which could otherwise block the extension on a full pipe.
func logOutput(ctx context.Context, name string, r io.Reader) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			log.G(ctx).WithField("extension", name).Info(strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			return
		}
	}
}

// declaredDependencies converts declared dependencies to extension dependencies.
func declaredDependencies(deps []sdkapi.Dependency) []extensions.Dependency {
	if len(deps) == 0 {
		return nil
	}
	out := make([]extensions.Dependency, 0, len(deps))
	for _, dep := range deps {
		out = append(out, extensions.Dependency{
			Point:     extensions.PointID(dep.Point),
			Extension: extensions.ExtensionID(dep.Extension),
			Optional:  dep.Optional,
		})
	}
	return out
}

// declaredConflicts converts declared conflict ids to extension ids.
func declaredConflicts(ids []string) []extensions.ExtensionID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]extensions.ExtensionID, 0, len(ids))
	for _, id := range ids {
		out = append(out, extensions.ExtensionID(id))
	}
	return out
}
