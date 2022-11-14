package runcexecutor

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/moby/buildkit/util/bklog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/containerd/containerd/mount"
	containerdoci "github.com/containerd/containerd/oci"
	"github.com/containerd/continuity/fs"
	runc "github.com/containerd/go-runc"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	gatewayapi "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	rootlessspecconv "github.com/moby/buildkit/util/rootless/specconv"
	"github.com/moby/buildkit/util/stack"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

type Opt struct {
	// root directory
	Root              string
	CommandCandidates []string
	// without root privileges (has nothing to do with Opt.Root directory)
	Rootless bool
	// DefaultCgroupParent is the cgroup-parent name for executor
	DefaultCgroupParent string
	// ProcessMode
	ProcessMode     oci.ProcessMode
	IdentityMapping *idtools.IdentityMapping
	// runc run --no-pivot (unrecommended)
	NoPivot         bool
	DNS             *oci.DNSConfig
	OOMScoreAdj     *int
	ApparmorProfile string
	SELinux         bool
	TracingSocket   string
}

var defaultCommandCandidates = []string{"buildkit-runc", "runc"}

type runcExecutor struct {
	runc             *runc.Runc
	root             string
	cgroupParent     string
	rootless         bool
	networkProviders map[pb.NetMode]network.Provider
	processMode      oci.ProcessMode
	idmap            *idtools.IdentityMapping
	noPivot          bool
	dns              *oci.DNSConfig
	oomScoreAdj      *int
	running          map[string]chan error
	mu               sync.Mutex
	apparmorProfile  string
	selinux          bool
	tracingSocket    string
}

func New(opt Opt, networkProviders map[pb.NetMode]network.Provider) (executor.Executor, error) {
	cmds := opt.CommandCandidates
	if cmds == nil {
		cmds = defaultCommandCandidates
	}

	var cmd string
	var found bool
	for _, cmd = range cmds {
		if _, err := exec.LookPath(cmd); err == nil {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.Errorf("failed to find %s binary", cmd)
	}

	root := opt.Root

	if err := os.MkdirAll(root, 0711); err != nil {
		return nil, errors.Wrapf(err, "failed to create %s", root)
	}

	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}

	// clean up old hosts/resolv.conf file. ignore errors
	os.RemoveAll(filepath.Join(root, "hosts"))
	os.RemoveAll(filepath.Join(root, "resolv.conf"))

	runtime := &runc.Runc{
		Command:   cmd,
		Log:       filepath.Join(root, "runc-log.json"),
		LogFormat: runc.JSON,
		Setpgid:   true,
		// we don't execute runc with --rootless=(true|false) explicitly,
		// so as to support non-runc runtimes
	}

	updateRuncFieldsForHostOS(runtime)

	w := &runcExecutor{
		runc:             runtime,
		root:             root,
		cgroupParent:     opt.DefaultCgroupParent,
		rootless:         opt.Rootless,
		networkProviders: networkProviders,
		processMode:      opt.ProcessMode,
		idmap:            opt.IdentityMapping,
		noPivot:          opt.NoPivot,
		dns:              opt.DNS,
		oomScoreAdj:      opt.OOMScoreAdj,
		running:          make(map[string]chan error),
		apparmorProfile:  opt.ApparmorProfile,
		selinux:          opt.SELinux,
		tracingSocket:    opt.TracingSocket,
	}
	return w, nil
}

func (w *runcExecutor) Run(ctx context.Context, id string, root executor.Mount, mounts []executor.Mount, process executor.ProcessInfo, started chan<- struct{}) (err error) {
	meta := process.Meta

	startedOnce := sync.Once{}
	done := make(chan error, 1)
	w.mu.Lock()
	w.running[id] = done
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.running, id)
		w.mu.Unlock()
		done <- err
		close(done)
		if started != nil {
			startedOnce.Do(func() {
				close(started)
			})
		}
	}()

	provider, ok := w.networkProviders[meta.NetMode]
	if !ok {
		return errors.Errorf("unknown network mode %s", meta.NetMode)
	}
	namespace, err := provider.New()
	if err != nil {
		return err
	}
	defer namespace.Close()

	if meta.NetMode == pb.NetMode_HOST {
		bklog.G(ctx).Info("enabling HostNetworking")
	}

	resolvConf, err := oci.GetResolvConf(ctx, w.root, w.idmap, w.dns)
	if err != nil {
		return err
	}

	hostsFile, clean, err := oci.GetHostsFile(ctx, w.root, meta.ExtraHosts, w.idmap, meta.Hostname)
	if err != nil {
		return err
	}
	if clean != nil {
		defer clean()
	}

	mountable, err := root.Src.Mount(ctx, false)
	if err != nil {
		return err
	}

	rootMount, release, err := mountable.Mount()
	if err != nil {
		return err
	}
	if release != nil {
		defer release()
	}

	if id == "" {
		id = identity.NewID()
	}
	bundle := filepath.Join(w.root, id)

	if err := os.Mkdir(bundle, 0711); err != nil {
		return err
	}
	defer os.RemoveAll(bundle)

	identity := idtools.Identity{}
	if w.idmap != nil {
		identity = w.idmap.RootPair()
	}

	rootFSPath := filepath.Join(bundle, "rootfs")
	if err := idtools.MkdirAllAndChown(rootFSPath, 0700, identity); err != nil {
		return err
	}
	if err := mount.All(rootMount, rootFSPath); err != nil {
		return err
	}
	defer mount.Unmount(rootFSPath, 0)

	defer executor.MountStubsCleaner(rootFSPath, mounts)()

	uid, gid, sgids, err := oci.GetUser(rootFSPath, meta.User)
	if err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(bundle, "config.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	opts := []containerdoci.SpecOpts{oci.WithUIDGID(uid, gid, sgids)}

	if meta.ReadonlyRootFS {
		opts = append(opts, containerdoci.WithRootFSReadonly())
	}

	identity = idtools.Identity{
		UID: int(uid),
		GID: int(gid),
	}
	if w.idmap != nil {
		identity, err = w.idmap.ToHost(identity)
		if err != nil {
			return err
		}
	}

	spec, cleanup, err := oci.GenerateSpec(ctx, meta, mounts, id, resolvConf, hostsFile, namespace, w.cgroupParent, w.processMode, w.idmap, w.apparmorProfile, w.selinux, w.tracingSocket, opts...)
	if err != nil {
		return err
	}
	defer cleanup()

	spec.Root.Path = rootFSPath
	if root.Readonly {
		spec.Root.Readonly = true
	}

	newp, err := fs.RootPath(rootFSPath, meta.Cwd)
	if err != nil {
		return errors.Wrapf(err, "working dir %s points to invalid target", newp)
	}
	if _, err := os.Stat(newp); err != nil {
		if err := idtools.MkdirAllAndChown(newp, 0755, identity); err != nil {
			return errors.Wrapf(err, "failed to create working directory %s", newp)
		}
	}

	spec.Process.Terminal = meta.Tty
	spec.Process.OOMScoreAdj = w.oomScoreAdj
	if w.rootless {
		if err := rootlessspecconv.ToRootless(spec); err != nil {
			return err
		}
	}

	if err := json.NewEncoder(f).Encode(spec); err != nil {
		return err
	}

	// runCtx/killCtx is used for extra check in case the kill command blocks
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	ended := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				killCtx, timeout := context.WithTimeout(context.Background(), 7*time.Second)
				if err := w.runc.Kill(killCtx, id, int(syscall.SIGKILL), nil); err != nil {
					bklog.G(ctx).Errorf("failed to kill runc %s: %+v", id, err)
					select {
					case <-killCtx.Done():
						timeout()
						cancelRun()
						return
					default:
					}
				}
				timeout()
				select {
				case <-time.After(50 * time.Millisecond):
				case <-ended:
					return
				}
			case <-ended:
				return
			}
		}
	}()

	bklog.G(ctx).Debugf("> creating %s %v", id, meta.Args)

	trace.SpanFromContext(ctx).AddEvent("Container created")
	err = w.run(runCtx, id, bundle, process, func() {
		startedOnce.Do(func() {
			trace.SpanFromContext(ctx).AddEvent("Container started")
			if started != nil {
				close(started)
			}
		})
	})
	close(ended)
	return exitError(ctx, err)
}

func exitError(ctx context.Context, err error) error {
	if err != nil {
		exitErr := &gatewayapi.ExitError{
			ExitCode: gatewayapi.UnknownExitStatus,
			Err:      err,
		}
		var runcExitError *runc.ExitError
		if errors.As(err, &runcExitError) {
			exitErr = &gatewayapi.ExitError{
				ExitCode: uint32(runcExitError.Status),
			}
		}
		trace.SpanFromContext(ctx).AddEvent(
			"Container exited",
			trace.WithAttributes(
				attribute.Int("exit.code", int(exitErr.ExitCode)),
			),
		)
		select {
		case <-ctx.Done():
			exitErr.Err = errors.Wrapf(ctx.Err(), exitErr.Error())
			return exitErr
		default:
			return stack.Enable(exitErr)
		}
	}

	trace.SpanFromContext(ctx).AddEvent(
		"Container exited",
		trace.WithAttributes(attribute.Int("exit.code", 0)),
	)
	return nil
}

func (w *runcExecutor) Exec(ctx context.Context, id string, process executor.ProcessInfo) (err error) {
	// first verify the container is running, if we get an error assume the container
	// is in the process of being created and check again every 100ms or until
	// context is canceled.
	var state *runc.Container
	for {
		w.mu.Lock()
		done, ok := w.running[id]
		w.mu.Unlock()
		if !ok {
			return errors.Errorf("container %s not found", id)
		}

		state, _ = w.runc.State(ctx, id)
		if state != nil && state.Status == "running" {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-done:
			if !ok || err == nil {
				return errors.Errorf("container %s has stopped", id)
			}
			return errors.Wrapf(err, "container %s has exited with error", id)
		case <-time.After(100 * time.Millisecond):
		}
	}

	// load default process spec (for Env, Cwd etc) from bundle
	f, err := os.Open(filepath.Join(state.Bundle, "config.json"))
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	spec := &specs.Spec{}
	if err := json.NewDecoder(f).Decode(spec); err != nil {
		return err
	}

	if process.Meta.User != "" {
		uid, gid, sgids, err := oci.GetUser(state.Rootfs, process.Meta.User)
		if err != nil {
			return err
		}
		spec.Process.User = specs.User{
			UID:            uid,
			GID:            gid,
			AdditionalGids: sgids,
		}
	}

	spec.Process.Terminal = process.Meta.Tty
	spec.Process.Args = process.Meta.Args
	if process.Meta.Cwd != "" {
		spec.Process.Cwd = process.Meta.Cwd
	}

	if len(process.Meta.Env) > 0 {
		spec.Process.Env = process.Meta.Env
	}

	err = w.exec(ctx, id, state.Bundle, spec.Process, process, nil)
	return exitError(ctx, err)
}

type forwardIO struct {
	stdin          io.ReadCloser
	stdout, stderr io.WriteCloser
}

func (s *forwardIO) Close() error {
	return nil
}

func (s *forwardIO) Set(cmd *exec.Cmd) {
	cmd.Stdin = s.stdin
	cmd.Stdout = s.stdout
	cmd.Stderr = s.stderr
}

func (s *forwardIO) Stdin() io.WriteCloser {
	return nil
}

func (s *forwardIO) Stdout() io.ReadCloser {
	return nil
}

func (s *forwardIO) Stderr() io.ReadCloser {
	return nil
}

// startingProcess is to track the os process so we can send signals to it.
type startingProcess struct {
	Process *os.Process
	ready   chan struct{}
}

// Release will free resources with a startingProcess.
func (p *startingProcess) Release() {
	if p.Process != nil {
		p.Process.Release()
	}
}

// WaitForReady will wait until the Process has been populated or the
// provided context was cancelled.  This should be called before using
// the Process field.
func (p *startingProcess) WaitForReady(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ready:
		return nil
	}
}

// WaitForStart will record the pid reported by Runc via the channel.
// We wait for up to 10s for the runc process to start.  If the started
// callback is non-nil it will be called after receiving the pid.
func (p *startingProcess) WaitForStart(ctx context.Context, startedCh <-chan int, started func()) error {
	startedCtx, timeout := context.WithTimeout(ctx, 10*time.Second)
	defer timeout()
	var err error
	select {
	case <-startedCtx.Done():
		return errors.New("runc started message never received")
	case pid, ok := <-startedCh:
		if !ok {
			return errors.New("runc process failed to send pid")
		}
		if started != nil {
			started()
		}
		p.Process, err = os.FindProcess(pid)
		if err != nil {
			return errors.Wrapf(err, "unable to find runc process for pid %d", pid)
		}
		close(p.ready)
	}
	return nil
}

// handleSignals will wait until the runcProcess is ready then will
// send each signal received on the channel to the process.
func handleSignals(ctx context.Context, runcProcess *startingProcess, signals <-chan syscall.Signal) error {
	if signals == nil {
		return nil
	}
	err := runcProcess.WaitForReady(ctx)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case sig := <-signals:
			err := runcProcess.Process.Signal(sig)
			if err != nil {
				bklog.G(ctx).Errorf("failed to signal %s to process: %s", sig, err)
				return err
			}
		}
	}
}
