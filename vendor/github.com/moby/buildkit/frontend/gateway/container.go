package gateway

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver/mounts"
	opspb "github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/stack"
	utilsystem "github.com/moby/buildkit/util/system"
	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type NewContainerRequest struct {
	ContainerID string
	NetMode     opspb.NetMode
	Mounts      []Mount
	Platform    *opspb.Platform
	Constraints *opspb.WorkerConstraints
}

// Mount used for the gateway.Container is nearly identical to the client.Mount
// except is has a RefProxy instead of Ref to allow for a common abstraction
// between gateway clients.
type Mount struct {
	Dest      string
	Selector  string
	Readonly  bool
	MountType opspb.MountType
	RefProxy  solver.ResultProxy
	CacheOpt  *opspb.CacheOpt
	SecretOpt *opspb.SecretOpt
	SSHOpt    *opspb.SSHOpt
}

func toProtoMount(m Mount) *opspb.Mount {
	return &opspb.Mount{
		Selector:  m.Selector,
		Dest:      m.Dest,
		Readonly:  m.Readonly,
		MountType: m.MountType,
		CacheOpt:  m.CacheOpt,
		SecretOpt: m.SecretOpt,
		SSHOpt:    m.SSHOpt,
	}
}

func NewContainer(ctx context.Context, e executor.Executor, sm *session.Manager, g session.Group, req NewContainerRequest) (client.Container, error) {
	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)
	platform := opspb.Platform{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
	if req.Platform != nil {
		platform = *req.Platform
	}
	ctr := &gatewayContainer{
		id:       req.ContainerID,
		netMode:  req.NetMode,
		platform: platform,
		executor: e,
		errGroup: eg,
		ctx:      ctx,
		cancel:   cancel,
	}

	makeMutable := func(worker worker.Worker, ref cache.ImmutableRef) (cache.MutableRef, error) {
		mRef, err := worker.CacheManager().New(ctx, ref, g)
		if err != nil {
			return nil, stack.Enable(err)
		}
		ctr.cleanup = append(ctr.cleanup, func() error {
			return stack.Enable(mRef.Release(context.TODO()))
		})
		return mRef, nil
	}

	var mm *mounts.MountManager
	mnts := req.Mounts

	for i, m := range mnts {
		if m.Dest == opspb.RootMount && m.RefProxy != nil {
			res, err := m.RefProxy.Result(ctx)
			if err != nil {
				return nil, stack.Enable(err)
			}
			workerRef, ok := res.Sys().(*worker.WorkerRef)
			if !ok {
				return nil, errors.Errorf("invalid reference for exec %T", res.Sys())
			}

			name := fmt.Sprintf("container %s", req.ContainerID)
			mm = mounts.NewMountManager(name, workerRef.Worker.CacheManager(), sm, workerRef.Worker.MetadataStore())

			ctr.rootFS = mountWithSession(workerRef.ImmutableRef, g)
			if !m.Readonly {
				ref, err := makeMutable(workerRef.Worker, workerRef.ImmutableRef)
				if err != nil {
					return nil, stack.Enable(err)
				}
				ctr.rootFS = mountWithSession(ref, g)
			}

			// delete root mount from list, handled here
			mnts = append(mnts[:i], mnts[i+1:]...)
			break
		}
	}

	if ctr.rootFS.Src == nil {
		return nil, errors.Errorf("root mount required")
	}

	for _, m := range mnts {
		var ref cache.ImmutableRef
		var mountable cache.Mountable
		if m.RefProxy != nil {
			res, err := m.RefProxy.Result(ctx)
			if err != nil {
				return nil, stack.Enable(err)
			}
			workerRef, ok := res.Sys().(*worker.WorkerRef)
			if !ok {
				return nil, errors.Errorf("invalid reference for exec %T", res.Sys())
			}
			ref = workerRef.ImmutableRef
			mountable = ref

			if !m.Readonly {
				mountable, err = makeMutable(workerRef.Worker, ref)
				if err != nil {
					return nil, stack.Enable(err)
				}
			}
		}
		switch m.MountType {
		case opspb.MountType_BIND:
			// nothing to do here
		case opspb.MountType_CACHE:
			mRef, err := mm.MountableCache(ctx, toProtoMount(m), ref, g)
			if err != nil {
				return nil, err
			}
			mountable = mRef
			ctr.cleanup = append(ctr.cleanup, func() error {
				return stack.Enable(mRef.Release(context.TODO()))
			})
		case opspb.MountType_TMPFS:
			mountable = mm.MountableTmpFS()
		case opspb.MountType_SECRET:
			var err error
			mountable, err = mm.MountableSecret(ctx, toProtoMount(m), g)
			if err != nil {
				return nil, err
			}
			if mountable == nil {
				continue
			}
		case opspb.MountType_SSH:
			var err error
			mountable, err = mm.MountableSSH(ctx, toProtoMount(m), g)
			if err != nil {
				return nil, err
			}
			if mountable == nil {
				continue
			}
		default:
			return nil, errors.Errorf("mount type %s not implemented", m.MountType)
		}

		// validate that there is a mount
		if mountable == nil {
			return nil, errors.Errorf("mount %s has no input", m.Dest)
		}

		execMount := executor.Mount{
			Src:      mountableWithSession(mountable, g),
			Selector: m.Selector,
			Dest:     m.Dest,
			Readonly: m.Readonly,
		}

		ctr.mounts = append(ctr.mounts, execMount)
	}

	// sort mounts so parents are mounted first
	sort.Slice(ctr.mounts, func(i, j int) bool {
		return ctr.mounts[i].Dest < ctr.mounts[j].Dest
	})

	return ctr, nil
}

type gatewayContainer struct {
	id       string
	netMode  opspb.NetMode
	platform opspb.Platform
	rootFS   executor.Mount
	mounts   []executor.Mount
	executor executor.Executor
	started  bool
	errGroup *errgroup.Group
	mu       sync.Mutex
	cleanup  []func() error
	ctx      context.Context
	cancel   func()
}

func (gwCtr *gatewayContainer) Start(ctx context.Context, req client.StartRequest) (client.ContainerProcess, error) {
	resize := make(chan executor.WinSize)
	procInfo := executor.ProcessInfo{
		Meta: executor.Meta{
			Args:         req.Args,
			Env:          req.Env,
			User:         req.User,
			Cwd:          req.Cwd,
			Tty:          req.Tty,
			NetMode:      gwCtr.netMode,
			SecurityMode: req.SecurityMode,
		},
		Stdin:  req.Stdin,
		Stdout: req.Stdout,
		Stderr: req.Stderr,
		Resize: resize,
	}
	if procInfo.Meta.Cwd == "" {
		procInfo.Meta.Cwd = "/"
	}
	procInfo.Meta.Env = addDefaultEnvvar(procInfo.Meta.Env, "PATH", utilsystem.DefaultPathEnv(gwCtr.platform.OS))
	if req.Tty {
		procInfo.Meta.Env = addDefaultEnvvar(procInfo.Meta.Env, "TERM", "xterm")
	}

	// mark that we have started on the first call to execProcess for this
	// container, so that future calls will call Exec rather than Run
	gwCtr.mu.Lock()
	started := gwCtr.started
	gwCtr.started = true
	gwCtr.mu.Unlock()

	eg, ctx := errgroup.WithContext(gwCtr.ctx)
	gwProc := &gatewayContainerProcess{
		resize:   resize,
		errGroup: eg,
		groupCtx: ctx,
	}

	if !started {
		startedCh := make(chan struct{})
		gwProc.errGroup.Go(func() error {
			logrus.Debugf("Starting new container for %s with args: %q", gwCtr.id, procInfo.Meta.Args)
			err := gwCtr.executor.Run(ctx, gwCtr.id, gwCtr.rootFS, gwCtr.mounts, procInfo, startedCh)
			return stack.Enable(err)
		})
		select {
		case <-ctx.Done():
		case <-startedCh:
		}
	} else {
		gwProc.errGroup.Go(func() error {
			logrus.Debugf("Execing into container %s with args: %q", gwCtr.id, procInfo.Meta.Args)
			err := gwCtr.executor.Exec(ctx, gwCtr.id, procInfo)
			return stack.Enable(err)
		})
	}

	gwCtr.errGroup.Go(gwProc.errGroup.Wait)

	return gwProc, nil
}

func (gwCtr *gatewayContainer) Release(ctx context.Context) error {
	gwCtr.cancel()
	err1 := gwCtr.errGroup.Wait()

	var err2 error
	for i := len(gwCtr.cleanup) - 1; i >= 0; i-- { // call in LIFO order
		err := gwCtr.cleanup[i]()
		if err2 == nil {
			err2 = err
		}
	}

	if err1 != nil {
		return stack.Enable(err1)
	}
	return stack.Enable(err2)
}

type gatewayContainerProcess struct {
	errGroup *errgroup.Group
	groupCtx context.Context
	resize   chan<- executor.WinSize
	mu       sync.Mutex
}

func (gwProc *gatewayContainerProcess) Wait() error {
	err := stack.Enable(gwProc.errGroup.Wait())
	gwProc.mu.Lock()
	defer gwProc.mu.Unlock()
	close(gwProc.resize)
	return err
}

func (gwProc *gatewayContainerProcess) Resize(ctx context.Context, size client.WinSize) error {
	gwProc.mu.Lock()
	defer gwProc.mu.Unlock()

	//  is the container done or should we proceed with sending event?
	select {
	case <-gwProc.groupCtx.Done():
		return nil
	case <-ctx.Done():
		return nil
	default:
	}

	// now we select on contexts again in case p.resize blocks b/c
	// container no longer reading from it.  In that case when
	// the errgroup finishes we want to unblock on the write
	// and exit
	select {
	case <-gwProc.groupCtx.Done():
	case <-ctx.Done():
	case gwProc.resize <- executor.WinSize{Cols: size.Cols, Rows: size.Rows}:
	}
	return nil
}

func addDefaultEnvvar(env []string, k, v string) []string {
	for _, e := range env {
		if strings.HasPrefix(e, k+"=") {
			return env
		}
	}
	return append(env, k+"="+v)
}

func mountWithSession(m cache.Mountable, g session.Group) executor.Mount {
	_, readonly := m.(cache.ImmutableRef)
	return executor.Mount{
		Src:      mountableWithSession(m, g),
		Readonly: readonly,
	}
}

func mountableWithSession(m cache.Mountable, g session.Group) executor.Mountable {
	return &mountable{m: m, g: g}
}

type mountable struct {
	m cache.Mountable
	g session.Group
}

func (m *mountable) Mount(ctx context.Context, readonly bool) (snapshot.Mountable, error) {
	return m.m.Mount(ctx, readonly, m.g)
}
