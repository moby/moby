package containerdexecutor

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/moby/buildkit/util/bklog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/mount"
	containerdoci "github.com/containerd/containerd/oci"
	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	gatewayapi "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	rootlessspecconv "github.com/moby/buildkit/util/rootless/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

type containerdExecutor struct {
	client           *containerd.Client
	root             string
	networkProviders map[pb.NetMode]network.Provider
	cgroupParent     string
	dnsConfig        *oci.DNSConfig
	running          map[string]chan error
	mu               sync.Mutex
	apparmorProfile  string
	traceSocket      string
	rootless         bool
}

// New creates a new executor backed by connection to containerd API
func New(client *containerd.Client, root, cgroup string, networkProviders map[pb.NetMode]network.Provider, dnsConfig *oci.DNSConfig, apparmorProfile string, traceSocket string, rootless bool) executor.Executor {
	// clean up old hosts/resolv.conf file. ignore errors
	os.RemoveAll(filepath.Join(root, "hosts"))
	os.RemoveAll(filepath.Join(root, "resolv.conf"))

	return &containerdExecutor{
		client:           client,
		root:             root,
		networkProviders: networkProviders,
		cgroupParent:     cgroup,
		dnsConfig:        dnsConfig,
		running:          make(map[string]chan error),
		apparmorProfile:  apparmorProfile,
		traceSocket:      traceSocket,
		rootless:         rootless,
	}
}

func (w *containerdExecutor) Run(ctx context.Context, id string, root executor.Mount, mounts []executor.Mount, process executor.ProcessInfo, started chan<- struct{}) (err error) {
	if id == "" {
		id = identity.NewID()
	}

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

	meta := process.Meta

	resolvConf, err := oci.GetResolvConf(ctx, w.root, nil, w.dnsConfig)
	if err != nil {
		return err
	}

	hostsFile, clean, err := oci.GetHostsFile(ctx, w.root, meta.ExtraHosts, nil, meta.Hostname)
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

	rootMounts, release, err := mountable.Mount()
	if err != nil {
		return err
	}
	if release != nil {
		defer release()
	}

	lm := snapshot.LocalMounterWithMounts(rootMounts)
	rootfsPath, err := lm.Mount()
	if err != nil {
		return err
	}
	defer lm.Unmount()
	defer executor.MountStubsCleaner(rootfsPath, mounts)()

	uid, gid, sgids, err := oci.GetUser(rootfsPath, meta.User)
	if err != nil {
		return err
	}

	identity := idtools.Identity{
		UID: int(uid),
		GID: int(gid),
	}

	newp, err := fs.RootPath(rootfsPath, meta.Cwd)
	if err != nil {
		return errors.Wrapf(err, "working dir %s points to invalid target", newp)
	}
	if _, err := os.Stat(newp); err != nil {
		if err := idtools.MkdirAllAndChown(newp, 0755, identity); err != nil {
			return errors.Wrapf(err, "failed to create working directory %s", newp)
		}
	}

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

	opts := []containerdoci.SpecOpts{oci.WithUIDGID(uid, gid, sgids)}
	if meta.ReadonlyRootFS {
		opts = append(opts, containerdoci.WithRootFSReadonly())
	}

	processMode := oci.ProcessSandbox // FIXME(AkihiroSuda)
	spec, cleanup, err := oci.GenerateSpec(ctx, meta, mounts, id, resolvConf, hostsFile, namespace, w.cgroupParent, processMode, nil, w.apparmorProfile, w.traceSocket, opts...)
	if err != nil {
		return err
	}
	defer cleanup()
	spec.Process.Terminal = meta.Tty
	if w.rootless {
		if err := rootlessspecconv.ToRootless(spec); err != nil {
			return err
		}
	}

	container, err := w.client.NewContainer(ctx, id,
		containerd.WithSpec(spec),
	)
	if err != nil {
		return err
	}

	defer func() {
		if err1 := container.Delete(context.TODO()); err == nil && err1 != nil {
			err = errors.Wrapf(err1, "failed to delete container %s", id)
		}
	}()

	fixProcessOutput(&process)
	cioOpts := []cio.Opt{cio.WithStreams(process.Stdin, process.Stdout, process.Stderr)}
	if meta.Tty {
		cioOpts = append(cioOpts, cio.WithTerminal)
	}

	task, err := container.NewTask(ctx, cio.NewCreator(cioOpts...), containerd.WithRootFS([]mount.Mount{{
		Source:  rootfsPath,
		Type:    "bind",
		Options: []string{"rbind"},
	}}))
	if err != nil {
		return err
	}

	defer func() {
		if _, err1 := task.Delete(context.TODO()); err == nil && err1 != nil {
			err = errors.Wrapf(err1, "failed to delete task %s", id)
		}
	}()

	trace.SpanFromContext(ctx).AddEvent("Container created")
	err = w.runProcess(ctx, task, process.Resize, process.Signal, func() {
		startedOnce.Do(func() {
			trace.SpanFromContext(ctx).AddEvent("Container started")
			if started != nil {
				close(started)
			}
		})
	})
	return err
}

func (w *containerdExecutor) Exec(ctx context.Context, id string, process executor.ProcessInfo) (err error) {
	meta := process.Meta

	// first verify the container is running, if we get an error assume the container
	// is in the process of being created and check again every 100ms or until
	// context is canceled.

	var container containerd.Container
	var task containerd.Task
	for {
		w.mu.Lock()
		done, ok := w.running[id]
		w.mu.Unlock()

		if !ok {
			return errors.Errorf("container %s not found", id)
		}

		if container == nil {
			container, _ = w.client.LoadContainer(ctx, id)
		}
		if container != nil && task == nil {
			task, _ = container.Task(ctx, nil)
		}
		if task != nil {
			status, _ := task.Status(ctx)
			if status.Status == containerd.Running {
				break
			}
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
			continue
		}
	}

	spec, err := container.Spec(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	proc := spec.Process

	// TODO how do we get rootfsPath for oci.GetUser in case user passed in username rather than uid:gid?
	// For now only support uid:gid
	if meta.User != "" {
		uid, gid, err := oci.ParseUIDGID(meta.User)
		if err != nil {
			return errors.WithStack(err)
		}
		proc.User = specs.User{
			UID:            uid,
			GID:            gid,
			AdditionalGids: []uint32{},
		}
	}

	proc.Terminal = meta.Tty
	proc.Args = meta.Args
	if meta.Cwd != "" {
		spec.Process.Cwd = meta.Cwd
	}
	if len(process.Meta.Env) > 0 {
		spec.Process.Env = process.Meta.Env
	}

	fixProcessOutput(&process)
	cioOpts := []cio.Opt{cio.WithStreams(process.Stdin, process.Stdout, process.Stderr)}
	if meta.Tty {
		cioOpts = append(cioOpts, cio.WithTerminal)
	}

	taskProcess, err := task.Exec(ctx, identity.NewID(), proc, cio.NewCreator(cioOpts...))
	if err != nil {
		return errors.WithStack(err)
	}

	err = w.runProcess(ctx, taskProcess, process.Resize, process.Signal, nil)
	return err
}

func fixProcessOutput(process *executor.ProcessInfo) {
	// It seems like if containerd has one of stdin, stdout or stderr then the
	// others need to be present as well otherwise we get this error:
	// failed to start io pipe copy: unable to copy pipes: containerd-shim: opening file "" failed: open : no such file or directory: unknown
	// So just stub out any missing output
	if process.Stdout == nil {
		process.Stdout = &nopCloser{ioutil.Discard}
	}
	if process.Stderr == nil {
		process.Stderr = &nopCloser{ioutil.Discard}
	}
}

func (w *containerdExecutor) runProcess(ctx context.Context, p containerd.Process, resize <-chan executor.WinSize, signal <-chan syscall.Signal, started func()) error {
	// Not using `ctx` here because the context passed only affects the statusCh which we
	// don't want cancelled when ctx.Done is sent.  We want to process statusCh on cancel.
	statusCh, err := p.Wait(context.Background())
	if err != nil {
		return err
	}

	io := p.IO()
	defer func() {
		io.Wait()
		io.Close()
	}()

	err = p.Start(ctx)
	if err != nil {
		return err
	}

	if started != nil {
		started()
	}

	p.CloseIO(ctx, containerd.WithStdinCloser)

	// handle signals (and resize) in separate go loop so it does not
	// potentially block the container cancel/exit status loop below.
	eventCtx, eventCancel := context.WithCancel(ctx)
	defer eventCancel()
	go func() {
		for {
			select {
			case <-eventCtx.Done():
				return
			case size, ok := <-resize:
				if !ok {
					return // chan closed
				}
				err = p.Resize(eventCtx, size.Cols, size.Rows)
				if err != nil {
					bklog.G(eventCtx).Warnf("Failed to resize %s: %s", p.ID(), err)
				}
			}
		}
	}()
	go func() {
		for {
			select {
			case <-eventCtx.Done():
				return
			case sig, ok := <-signal:
				if !ok {
					return // chan closed
				}
				err = p.Kill(eventCtx, sig)
				if err != nil {
					bklog.G(eventCtx).Warnf("Failed to signal %s: %s", p.ID(), err)
				}
			}
		}
	}()

	var cancel func()
	var killCtxDone <-chan struct{}
	ctxDone := ctx.Done()
	for {
		select {
		case <-ctxDone:
			ctxDone = nil
			var killCtx context.Context
			killCtx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
			killCtxDone = killCtx.Done()
			p.Kill(killCtx, syscall.SIGKILL)
			io.Cancel()
		case status := <-statusCh:
			if cancel != nil {
				cancel()
			}
			trace.SpanFromContext(ctx).AddEvent(
				"Container exited",
				trace.WithAttributes(
					attribute.Int("exit.code", int(status.ExitCode())),
				),
			)
			if status.ExitCode() != 0 {
				exitErr := &gatewayapi.ExitError{
					ExitCode: status.ExitCode(),
					Err:      status.Error(),
				}
				if status.ExitCode() == gatewayapi.UnknownExitStatus && status.Error() != nil {
					exitErr.Err = errors.Wrap(status.Error(), "failure waiting for process")
				}
				select {
				case <-ctx.Done():
					exitErr.Err = errors.Wrap(ctx.Err(), exitErr.Error())
				default:
				}
				return exitErr
			}
			return nil
		case <-killCtxDone:
			if cancel != nil {
				cancel()
			}
			io.Cancel()
			return errors.Errorf("failed to kill process on cancel")
		}
	}
}

type nopCloser struct {
	io.Writer
}

func (c *nopCloser) Close() error {
	return nil
}
