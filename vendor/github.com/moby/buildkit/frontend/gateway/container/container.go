package container

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/moby/buildkit/session/secrets"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/system"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/llbsolver/mounts"
	"github.com/moby/buildkit/solver/pb"
	opspb "github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/stack"
	utilsystem "github.com/moby/buildkit/util/system"
	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type NewContainerRequest struct {
	ContainerID string
	NetMode     opspb.NetMode
	Hostname    string
	ExtraHosts  []executor.HostIP
	Mounts      []Mount
	Platform    *opspb.Platform
	Constraints *opspb.WorkerConstraints
}

// Mount used for the gateway.Container is nearly identical to the client.Mount
// except is has a RefProxy instead of Ref to allow for a common abstraction
// between gateway clients.
type Mount struct {
	*opspb.Mount
	WorkerRef *worker.WorkerRef
}

func NewContainer(ctx context.Context, cm cache.Manager, exec executor.Executor, sm *session.Manager, g session.Group, req NewContainerRequest) (client.Container, error) {
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
		id:         req.ContainerID,
		netMode:    req.NetMode,
		hostname:   req.Hostname,
		extraHosts: req.ExtraHosts,
		platform:   platform,
		executor:   exec,
		sm:         sm,
		group:      g,
		errGroup:   eg,
		ctx:        ctx,
		cancel:     cancel,
	}

	var (
		mnts []*opspb.Mount
		refs []*worker.WorkerRef
	)
	for _, m := range req.Mounts {
		mnts = append(mnts, m.Mount)
		if m.WorkerRef != nil {
			refs = append(refs, m.WorkerRef)
			m.Mount.Input = opspb.InputIndex(len(refs) - 1)
		} else {
			m.Mount.Input = opspb.Empty
		}
	}

	name := fmt.Sprintf("container %s", req.ContainerID)
	mm := mounts.NewMountManager(name, cm, sm)
	p, err := PrepareMounts(ctx, mm, cm, g, "", mnts, refs, func(m *opspb.Mount, ref cache.ImmutableRef) (cache.MutableRef, error) {
		if m.Input != opspb.Empty {
			cm = refs[m.Input].Worker.CacheManager()
		}
		return cm.New(ctx, ref, g)
	}, platform.OS)
	if err != nil {
		for i := len(p.Actives) - 1; i >= 0; i-- { // call in LIFO order
			p.Actives[i].Ref.Release(context.TODO())
		}
		for _, o := range p.OutputRefs {
			o.Ref.Release(context.TODO())
		}
		return nil, err
	}
	ctr.rootFS = p.Root
	ctr.mounts = p.Mounts

	for _, o := range p.OutputRefs {
		o := o
		ctr.cleanup = append(ctr.cleanup, func() error {
			return o.Ref.Release(context.TODO())
		})
	}
	for _, active := range p.Actives {
		active := active
		ctr.cleanup = append(ctr.cleanup, func() error {
			return active.Ref.Release(context.TODO())
		})
	}

	return ctr, nil
}

type PreparedMounts struct {
	Root           executor.Mount
	ReadonlyRootFS bool
	Mounts         []executor.Mount
	OutputRefs     []MountRef
	Actives        []MountMutableRef
}

type MountRef struct {
	Ref        cache.Ref
	MountIndex int
}

type MountMutableRef struct {
	Ref        cache.MutableRef
	MountIndex int
	NoCommit   bool
}

type MakeMutable func(m *opspb.Mount, ref cache.ImmutableRef) (cache.MutableRef, error)

func PrepareMounts(ctx context.Context, mm *mounts.MountManager, cm cache.Manager, g session.Group, cwd string, mnts []*opspb.Mount, refs []*worker.WorkerRef, makeMutable MakeMutable, platform string) (p PreparedMounts, err error) {
	// loop over all mounts, fill in mounts, root and outputs
	for i, m := range mnts {
		var (
			mountable cache.Mountable
			ref       cache.ImmutableRef
		)

		if m.Dest == opspb.RootMount && m.MountType != opspb.MountType_BIND {
			return p, errors.Errorf("invalid mount type %s for %s", m.MountType.String(), m.Dest)
		}

		// if mount is based on input validate and load it
		if m.Input != opspb.Empty {
			if int(m.Input) >= len(refs) {
				return p, errors.Errorf("missing input %d", m.Input)
			}
			ref = refs[int(m.Input)].ImmutableRef
			mountable = ref
		}

		switch m.MountType {
		case opspb.MountType_BIND:
			// if mount creates an output
			if m.Output != opspb.SkipOutput {
				// if it is readonly and not root then output is the input
				if m.Readonly && ref != nil && m.Dest != opspb.RootMount {
					p.OutputRefs = append(p.OutputRefs, MountRef{
						MountIndex: i,
						Ref:        ref.Clone(),
					})
				} else {
					// otherwise output and mount is the mutable child
					active, err := makeMutable(m, ref)
					if err != nil {
						return p, err
					}
					mountable = active
					p.OutputRefs = append(p.OutputRefs, MountRef{
						MountIndex: i,
						Ref:        active,
					})
				}
			} else if (!m.Readonly || ref == nil) && m.Dest != opspb.RootMount {
				// this case is empty readonly scratch without output that is not really useful for anything but don't error
				active, err := makeMutable(m, ref)
				if err != nil {
					return p, err
				}
				p.Actives = append(p.Actives, MountMutableRef{
					MountIndex: i,
					Ref:        active,
				})
				mountable = active
			}

		case opspb.MountType_CACHE:
			active, err := mm.MountableCache(ctx, m, ref, g)
			if err != nil {
				return p, err
			}
			mountable = active
			p.Actives = append(p.Actives, MountMutableRef{
				MountIndex: i,
				Ref:        active,
				NoCommit:   true,
			})
			if m.Output != opspb.SkipOutput && ref != nil {
				p.OutputRefs = append(p.OutputRefs, MountRef{
					MountIndex: i,
					Ref:        ref.Clone(),
				})
			}

		case opspb.MountType_TMPFS:
			mountable = mm.MountableTmpFS(m)
		case opspb.MountType_SECRET:
			var err error
			mountable, err = mm.MountableSecret(ctx, m, g)
			if err != nil {
				return p, err
			}
			if mountable == nil {
				continue
			}
		case opspb.MountType_SSH:
			var err error
			mountable, err = mm.MountableSSH(ctx, m, g)
			if err != nil {
				return p, err
			}
			if mountable == nil {
				continue
			}

		default:
			return p, errors.Errorf("mount type %s not implemented", m.MountType)
		}

		// validate that there is a mount
		if mountable == nil {
			return p, errors.Errorf("mount %s has no input", m.Dest)
		}

		// if dest is root we need mutable ref even if there is no output
		if m.Dest == opspb.RootMount {
			root := mountable
			p.ReadonlyRootFS = m.Readonly
			if m.Output == opspb.SkipOutput && p.ReadonlyRootFS {
				active, err := makeMutable(m, ref)
				if err != nil {
					return p, err
				}
				p.Actives = append(p.Actives, MountMutableRef{
					MountIndex: i,
					Ref:        active,
				})
				root = active
			}
			p.Root = MountWithSession(root, g)
		} else {
			mws := MountWithSession(mountable, g)
			dest := m.Dest
			if !system.IsAbs(filepath.Clean(dest), platform) {
				dest = filepath.Join("/", cwd, dest)
			}
			mws.Dest = dest
			mws.Readonly = m.Readonly
			mws.Selector = m.Selector
			p.Mounts = append(p.Mounts, mws)
		}
	}

	// sort mounts so parents are mounted first
	sort.Slice(p.Mounts, func(i, j int) bool {
		return p.Mounts[i].Dest < p.Mounts[j].Dest
	})

	return p, nil
}

type gatewayContainer struct {
	id         string
	netMode    opspb.NetMode
	hostname   string
	extraHosts []executor.HostIP
	platform   opspb.Platform
	rootFS     executor.Mount
	mounts     []executor.Mount
	executor   executor.Executor
	sm         *session.Manager
	group      session.Group
	started    bool
	errGroup   *errgroup.Group
	mu         sync.Mutex
	cleanup    []func() error
	ctx        context.Context
	cancel     func()
}

func (gwCtr *gatewayContainer) Start(ctx context.Context, req client.StartRequest) (client.ContainerProcess, error) {
	resize := make(chan executor.WinSize)
	signal := make(chan syscall.Signal)
	procInfo := executor.ProcessInfo{
		Meta: executor.Meta{
			Args:                      req.Args,
			Env:                       req.Env,
			User:                      req.User,
			Cwd:                       req.Cwd,
			Tty:                       req.Tty,
			NetMode:                   gwCtr.netMode,
			Hostname:                  gwCtr.hostname,
			ExtraHosts:                gwCtr.extraHosts,
			SecurityMode:              req.SecurityMode,
			RemoveMountStubsRecursive: req.RemoveMountStubsRecursive,
		},
		Stdin:  req.Stdin,
		Stdout: req.Stdout,
		Stderr: req.Stderr,
		Resize: resize,
		Signal: signal,
	}
	if procInfo.Meta.Cwd == "" {
		procInfo.Meta.Cwd = "/"
	}
	procInfo.Meta.Env = addDefaultEnvvar(procInfo.Meta.Env, "PATH", utilsystem.DefaultPathEnv(gwCtr.platform.OS))
	if req.Tty {
		procInfo.Meta.Env = addDefaultEnvvar(procInfo.Meta.Env, "TERM", "xterm")
	}

	secretEnv, err := gwCtr.loadSecretEnv(ctx, req.SecretEnv)
	if err != nil {
		return nil, err
	}
	procInfo.Meta.Env = append(procInfo.Meta.Env, secretEnv...)

	// mark that we have started on the first call to execProcess for this
	// container, so that future calls will call Exec rather than Run
	gwCtr.mu.Lock()
	started := gwCtr.started
	gwCtr.started = true
	gwCtr.mu.Unlock()

	eg, ctx := errgroup.WithContext(gwCtr.ctx)
	gwProc := &gatewayContainerProcess{
		resize:   resize,
		signal:   signal,
		errGroup: eg,
		groupCtx: ctx,
	}

	if !started {
		startedCh := make(chan struct{})
		gwProc.errGroup.Go(func() error {
			bklog.G(gwCtr.ctx).Debugf("Starting new container for %s with args: %q", gwCtr.id, procInfo.Meta.Args)
			_, err := gwCtr.executor.Run(ctx, gwCtr.id, gwCtr.rootFS, gwCtr.mounts, procInfo, startedCh)
			return stack.Enable(err)
		})
		select {
		case <-ctx.Done():
		case <-startedCh:
		}
	} else {
		gwProc.errGroup.Go(func() error {
			bklog.G(gwCtr.ctx).Debugf("Execing into container %s with args: %q", gwCtr.id, procInfo.Meta.Args)
			err := gwCtr.executor.Exec(ctx, gwCtr.id, procInfo)
			return stack.Enable(err)
		})
	}

	gwCtr.errGroup.Go(gwProc.errGroup.Wait)

	return gwProc, nil
}

func (gwCtr *gatewayContainer) loadSecretEnv(ctx context.Context, secretEnv []*pb.SecretEnv) ([]string, error) {
	out := make([]string, 0, len(secretEnv))
	for _, sopt := range secretEnv {
		id := sopt.ID
		if id == "" {
			return nil, errors.Errorf("secret ID missing for %q environment variable", sopt.Name)
		}
		var dt []byte
		var err error
		err = gwCtr.sm.Any(ctx, gwCtr.group, func(ctx context.Context, _ string, caller session.Caller) error {
			dt, err = secrets.GetSecret(ctx, caller, id)
			if err != nil {
				if errors.Is(err, secrets.ErrNotFound) && sopt.Optional {
					return nil
				}
				return err
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("%s=%s", sopt.Name, string(dt)))
	}
	return out, nil
}

func (gwCtr *gatewayContainer) Release(ctx context.Context) error {
	gwCtr.mu.Lock()
	defer gwCtr.mu.Unlock()
	gwCtr.cancel()
	err1 := gwCtr.errGroup.Wait()

	var err2 error
	for i := len(gwCtr.cleanup) - 1; i >= 0; i-- { // call in LIFO order
		err := gwCtr.cleanup[i]()
		if err2 == nil {
			err2 = err
		}
	}
	gwCtr.cleanup = nil

	if err1 != nil {
		return stack.Enable(err1)
	}
	return stack.Enable(err2)
}

type gatewayContainerProcess struct {
	errGroup *errgroup.Group
	groupCtx context.Context
	resize   chan<- executor.WinSize
	signal   chan<- syscall.Signal
	mu       sync.Mutex
}

func (gwProc *gatewayContainerProcess) Wait() error {
	err := stack.Enable(gwProc.errGroup.Wait())
	gwProc.mu.Lock()
	defer gwProc.mu.Unlock()
	close(gwProc.resize)
	close(gwProc.signal)
	return err
}

func (gwProc *gatewayContainerProcess) Resize(ctx context.Context, size client.WinSize) error {
	gwProc.mu.Lock()
	defer gwProc.mu.Unlock()

	// is the container done or should we proceed with sending event?
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

func (gwProc *gatewayContainerProcess) Signal(ctx context.Context, sig syscall.Signal) error {
	gwProc.mu.Lock()
	defer gwProc.mu.Unlock()

	// is the container done or should we proceed with sending event?
	select {
	case <-gwProc.groupCtx.Done():
		return nil
	case <-ctx.Done():
		return nil
	default:
	}

	// now we select on contexts again in case p.signal blocks b/c
	// container no longer reading from it.  In that case when
	// the errgroup finishes we want to unblock on the write
	// and exit
	select {
	case <-gwProc.groupCtx.Done():
	case <-ctx.Done():
	case gwProc.signal <- sig:
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

func MountWithSession(m cache.Mountable, g session.Group) executor.Mount {
	_, readonly := m.(cache.ImmutableRef)
	return executor.Mount{
		Src:      &mountable{m: m, g: g},
		Readonly: readonly,
	}
}

type mountable struct {
	m cache.Mountable
	g session.Group
}

func (m *mountable) Mount(ctx context.Context, readonly bool) (snapshot.Mountable, error) {
	return m.m.Mount(ctx, readonly, m.g)
}
