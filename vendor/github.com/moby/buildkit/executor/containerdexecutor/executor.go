package containerdexecutor

import (
	"context"
	"io"
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
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	gatewayapi "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	"github.com/pkg/errors"
)

type containerdExecutor struct {
	client           *containerd.Client
	root             string
	networkProviders map[pb.NetMode]network.Provider
	cgroupParent     string
	dnsConfig        *oci.DNSConfig
	running          map[string]*containerState
	mu               sync.Mutex
	apparmorProfile  string
	selinux          bool
	traceSocket      string
	rootless         bool
	runtime          *RuntimeInfo
}

// OnCreateRuntimer provides an alternative to OCI hooks for applying network
// configuration to a container. If the [network.Provider] returns a
// [network.Namespace] which also implements this interface, the containerd
// executor will run the callback at the appropriate point in the container
// lifecycle.
type OnCreateRuntimer interface {
	// OnCreateRuntime is analogous to the createRuntime OCI hook. The
	// function is called after the container is created, before the user
	// process has been executed. The argument is the container PID in the
	// runtime namespace.
	OnCreateRuntime(pid uint32) error
}

type RuntimeInfo struct {
	Name    string
	Path    string
	Options any
}

type ExecutorOptions struct {
	Client           *containerd.Client
	Root             string
	CgroupParent     string
	NetworkProviders map[pb.NetMode]network.Provider
	DNSConfig        *oci.DNSConfig
	ApparmorProfile  string
	Selinux          bool
	TraceSocket      string
	Rootless         bool
	Runtime          *RuntimeInfo
}

// New creates a new executor backed by connection to containerd API
func New(executorOpts ExecutorOptions) executor.Executor {
	// clean up old hosts/resolv.conf file. ignore errors
	os.RemoveAll(filepath.Join(executorOpts.Root, "hosts"))
	os.RemoveAll(filepath.Join(executorOpts.Root, "resolv.conf"))

	return &containerdExecutor{
		client:           executorOpts.Client,
		root:             executorOpts.Root,
		networkProviders: executorOpts.NetworkProviders,
		cgroupParent:     executorOpts.CgroupParent,
		dnsConfig:        executorOpts.DNSConfig,
		running:          make(map[string]*containerState),
		apparmorProfile:  executorOpts.ApparmorProfile,
		selinux:          executorOpts.Selinux,
		traceSocket:      executorOpts.TraceSocket,
		rootless:         executorOpts.Rootless,
		runtime:          executorOpts.Runtime,
	}
}

type containerState struct {
	done chan error
	// On linux the rootfsPath is used to ensure the CWD exists, to fetch user information
	// and as a bind mount for the root FS of the container.
	rootfsPath string
	// On Windows we need to use the root mounts to achieve the same thing that Linux does
	// with rootfsPath. So we save both in details.
	rootMounts []mount.Mount
}

func (w *containerdExecutor) Run(ctx context.Context, id string, root executor.Mount, mounts []executor.Mount, process executor.ProcessInfo, started chan<- struct{}) (rec resourcestypes.Recorder, err error) {
	if id == "" {
		id = identity.NewID()
	}

	startedOnce := sync.Once{}
	done := make(chan error, 1)
	details := &containerState{
		done: done,
	}
	w.mu.Lock()
	w.running[id] = details
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
	if meta.NetMode == pb.NetMode_HOST {
		bklog.G(ctx).Info("enabling HostNetworking")
	}

	provider, ok := w.networkProviders[meta.NetMode]
	if !ok {
		return nil, errors.Errorf("unknown network mode %s", meta.NetMode)
	}

	resolvConf, hostsFile, releasers, err := w.prepareExecutionEnv(ctx, root, mounts, meta, details, meta.NetMode)
	if err != nil {
		return nil, err
	}

	if releasers != nil {
		defer releasers()
	}

	if err := w.ensureCWD(ctx, details, meta); err != nil {
		return nil, err
	}

	namespace, err := provider.New(ctx, meta.Hostname)
	if err != nil {
		return nil, err
	}
	defer namespace.Close()

	spec, releaseSpec, err := w.createOCISpec(ctx, id, resolvConf, hostsFile, namespace, mounts, meta, details)
	if err != nil {
		return nil, err
	}
	if releaseSpec != nil {
		defer releaseSpec()
	}

	opts := []containerd.NewContainerOpts{
		containerd.WithSpec(spec),
	}
	if w.runtime != nil {
		opts = append(opts, containerd.WithRuntime(w.runtime.Name, w.runtime.Options))
	}
	container, err := w.client.NewContainer(ctx, id, opts...)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err1 := container.Delete(context.WithoutCancel(ctx)); err == nil && err1 != nil {
			err = errors.Wrapf(err1, "failed to delete container %s", id)
		}
	}()

	fixProcessOutput(&process)
	cioOpts := []cio.Opt{cio.WithStreams(process.Stdin, process.Stdout, process.Stderr)}
	if meta.Tty {
		cioOpts = append(cioOpts, cio.WithTerminal)
	}

	taskOpts, err := details.getTaskOpts()
	if err != nil {
		return nil, err
	}
	if w.runtime != nil && w.runtime.Path != "" {
		taskOpts = append(taskOpts, containerd.WithRuntimePath(w.runtime.Path))
	}
	task, err := container.NewTask(ctx, cio.NewCreator(cioOpts...), taskOpts...)
	if err != nil {
		return nil, err
	}

	defer func() {
		if _, err1 := task.Delete(context.WithoutCancel(ctx), containerd.WithProcessKill); err == nil && err1 != nil {
			err = errors.Wrapf(err1, "failed to delete task %s", id)
		}
	}()

	if nn, ok := namespace.(OnCreateRuntimer); ok {
		if err := nn.OnCreateRuntime(task.Pid()); err != nil {
			return nil, err
		}
	}

	trace.SpanFromContext(ctx).AddEvent("Container created")
	err = w.runProcess(ctx, task, process.Resize, process.Signal, func() {
		startedOnce.Do(func() {
			trace.SpanFromContext(ctx).AddEvent("Container started")
			if started != nil {
				close(started)
			}
		})
	})
	return nil, err
}

func (w *containerdExecutor) Exec(ctx context.Context, id string, process executor.ProcessInfo) (err error) {
	meta := process.Meta

	// first verify the container is running, if we get an error assume the container
	// is in the process of being created and check again every 100ms or until
	// context is canceled.

	w.mu.Lock()
	details, ok := w.running[id]
	w.mu.Unlock()

	if !ok {
		return errors.Errorf("container %s not found", id)
	}
	var container containerd.Container
	var task containerd.Task
	for {
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
			return context.Cause(ctx)
		case err, ok := <-details.done:
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
	if meta.User != "" {
		userSpec, err := getUserSpec(meta.User, details.rootfsPath)
		if err != nil {
			return errors.WithStack(err)
		}
		proc.User = userSpec
	}

	proc.Terminal = meta.Tty
	// setArgs will set the proper command line arguments for this process.
	// On Windows, this will set the CommandLine field. On Linux it will set the
	// Args field.
	setArgs(proc, meta.Args)

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
		process.Stdout = &nopCloser{io.Discard}
	}
	if process.Stderr == nil {
		process.Stderr = &nopCloser{io.Discard}
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
	eventCtx, eventCancel := context.WithCancelCause(ctx)
	defer eventCancel(errors.WithStack(context.Canceled))
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

	var cancel func(error)
	var killCtxDone <-chan struct{}
	ctxDone := ctx.Done()
	for {
		select {
		case <-ctxDone:
			ctxDone = nil
			var killCtx context.Context
			killCtx, cancel = context.WithCancelCause(context.Background())
			killCtx, _ = context.WithTimeoutCause(killCtx, 10*time.Second, errors.WithStack(context.DeadlineExceeded))
			killCtxDone = killCtx.Done()
			p.Kill(killCtx, syscall.SIGKILL)
			io.Cancel()
		case status := <-statusCh:
			if cancel != nil {
				cancel(errors.WithStack(context.Canceled))
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
					exitErr.Err = errors.Wrap(context.Cause(ctx), exitErr.Error())
				default:
				}
				return exitErr
			}
			return nil
		case <-killCtxDone:
			if cancel != nil {
				cancel(errors.WithStack(context.Canceled))
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
