package container

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	opspb "github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/stack"
	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"
	fstypes "github.com/tonistiigi/fsutil/types"
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
	ctx, cancel := context.WithCancelCause(ctx)
	eg, ctx := errgroup.WithContext(ctx)
	platform := &opspb.Platform{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
	if req.Platform != nil {
		platform = req.Platform
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
			m.Input = int64(len(refs) - 1)
		} else {
			m.Input = int64(opspb.Empty)
		}
	}

	name := fmt.Sprintf("container %s", req.ContainerID)
	mm := mounts.NewMountManager(name, cm, sm)
	p, err := PrepareMounts(ctx, mm, cm, g, "", mnts, refs, func(m *opspb.Mount, ref cache.ImmutableRef) (cache.MutableRef, error) {
		if m.Input != int64(opspb.Empty) {
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

	// Setup the local mounts.
	ctr.localMounts = setupLocalMounts(mnts, p)

	for _, o := range p.OutputRefs {
		ctr.cleanup = append(ctr.cleanup, func() error {
			return o.Ref.Release(context.TODO())
		})
	}
	for _, active := range p.Actives {
		ctr.cleanup = append(ctr.cleanup, func() error {
			return active.Ref.Release(context.TODO())
		})
	}

	return ctr, nil
}

// setupLocalMounts will setup the local mounts from the prepared mounts. These need
// to be in the same order as the original parameters.
func setupLocalMounts(mnts []*opspb.Mount, p PreparedMounts) []gatewayContainerMount {
	var mountableByDest map[string]executor.Mountable
	if len(p.Mounts) > 0 {
		mountableByDest = make(map[string]executor.Mountable, len(p.Mounts))
		for _, m := range p.Mounts {
			mountableByDest[m.Dest] = m.Src
		}
	}

	localMounts := make([]gatewayContainerMount, len(mnts))
	for i, m := range mnts {
		if m.Dest == "/" {
			localMounts[i].Src = p.Root.Src
			continue
		}
		localMounts[i].Src = mountableByDest[m.Dest]
	}
	return localMounts
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
		if m.Input != int64(opspb.Empty) {
			if int(m.Input) >= len(refs) {
				return p, errors.Errorf("missing input %d", m.Input)
			}
			ref = refs[int(m.Input)].ImmutableRef
			mountable = ref
		}

		switch m.MountType {
		case opspb.MountType_BIND:
			// if mount creates an output
			if m.Output != int64(opspb.SkipOutput) {
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
			if m.Output != int64(opspb.SkipOutput) && ref != nil {
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
			if m.Output == int64(opspb.SkipOutput) && p.ReadonlyRootFS {
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
	slices.SortFunc(p.Mounts, func(a, b executor.Mount) int {
		return cmp.Compare(a.Dest, b.Dest)
	})

	return p, nil
}

type gatewayContainer struct {
	id          string
	netMode     opspb.NetMode
	hostname    string
	extraHosts  []executor.HostIP
	platform    *opspb.Platform
	rootFS      executor.Mount
	mounts      []executor.Mount
	executor    executor.Executor
	sm          *session.Manager
	group       session.Group
	started     bool
	errGroup    *errgroup.Group
	mu          sync.Mutex
	cleanup     []func() error
	ctx         context.Context
	cancel      func(error)
	localMounts []gatewayContainerMount
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
	procInfo.Meta.Env = addDefaultEnvvar(procInfo.Meta.Env, "PATH", system.DefaultPathEnv(gwCtr.platform.OS))
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

func (gwCtr *gatewayContainer) loadSecretEnv(ctx context.Context, secretEnv []*opspb.SecretEnv) ([]string, error) {
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
				return err
			}
			return nil
		})
		if err != nil && (!errors.Is(err, secrets.ErrNotFound) || !sopt.Optional) {
			return nil, err
		}
		out = append(out, fmt.Sprintf("%s=%s", sopt.Name, string(dt)))
	}
	return out, nil
}

func (gwCtr *gatewayContainer) Release(ctx context.Context) error {
	gwCtr.mu.Lock()
	defer gwCtr.mu.Unlock()
	gwCtr.cancel(errors.WithStack(context.Canceled))
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

func (gwCtr *gatewayContainer) ReadFile(ctx context.Context, req client.ReadContainerRequest) ([]byte, error) {
	fsys, err := gwCtr.mount(ctx, req.MountIndex)
	if err != nil {
		return nil, err
	}

	path, err := filepath.Rel("/", req.Filename)
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(fsys, path)
}

func (gwCtr *gatewayContainer) ReadDir(ctx context.Context, req client.ReadDirContainerRequest) ([]*fstypes.Stat, error) {
	fsys, err := gwCtr.mount(ctx, req.MountIndex)
	if err != nil {
		return nil, err
	}

	path, err := filepath.Rel("/", req.Path)
	if err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(fsys, path)
	if err != nil {
		return nil, err
	}

	files := make([]*fstypes.Stat, len(entries))
	for i, e := range entries {
		fullpath := filepath.Join(req.Path, e.Name())
		fi, err := e.Info()
		if err != nil {
			return nil, err
		}

		files[i], err = mkstat(fsys, fullpath, e.Name(), fi)
		if err != nil {
			return nil, errors.Wrap(err, "mkstat")
		}
	}
	return files, nil
}

func (gwCtr *gatewayContainer) StatFile(ctx context.Context, req client.StatContainerRequest) (*fstypes.Stat, error) {
	fsys, err := gwCtr.mount(ctx, req.MountIndex)
	if err != nil {
		return nil, err
	}

	path, err := filepath.Rel("/", req.Path)
	if err != nil {
		return nil, err
	}

	fi, err := fs.Stat(fsys, path)
	if err != nil {
		return nil, err
	}
	return mkstat(fsys, req.Path, filepath.Base(req.Path), fi)
}

func (gwCtr *gatewayContainer) mount(ctx context.Context, index int) (fs.FS, error) {
	// No lock needed for this because the number of mounts does
	// not change.
	if index < 0 || index >= len(gwCtr.localMounts) {
		return nil, errors.Errorf("mount index %d is out of bounds (%d available)", index, len(gwCtr.localMounts))
	}

	gwCtr.mu.Lock()
	defer gwCtr.mu.Unlock()

	mount := gwCtr.localMounts[index]

	// Already mounted?
	if mount.FS != nil {
		return mount.FS, nil
	}

	// Defensively check that this mount really exists.
	if mount.Src == nil {
		return nil, errors.Errorf("mountable %d not found", index)
	}

	// Need to mount an instance.
	ref, err := mount.Src.Mount(ctx, true)
	if err != nil {
		return nil, err
	}

	mounter := snapshot.LocalMounter(ref)
	dir, err := mounter.Mount()
	if err != nil {
		return nil, err
	}

	// Register cleanup.
	gwCtr.cleanup = append(gwCtr.cleanup, func() error {
		return mounter.Unmount()
	})

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}

	gwCtr.cleanup = append(gwCtr.cleanup, func() error {
		return root.Close()
	})

	f := root.FS()
	gwCtr.localMounts[index].FS = f
	return f, nil
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

// constructs a Stat object. path is where the path can be found right
// now, relpath is the desired path to be recorded in the stat (so
// relative to whatever base dir is relevant). fi is the os.Stat
// info. inodemap is used to calculate hardlinks over a series of
// mkstat calls and maps inode to the canonical (aka "first") path for
// a set of hardlinks to that inode.
func mkstat(fsys fs.FS, path, relpath string, fi os.FileInfo) (*fstypes.Stat, error) {
	relpath = filepath.ToSlash(relpath)

	stat := &fstypes.Stat{
		Path:    filepath.FromSlash(relpath),
		Mode:    uint32(fi.Mode()),
		ModTime: fi.ModTime().UnixNano(),
	}

	if !fi.IsDir() {
		stat.Size = fi.Size()
		if fi.Mode()&os.ModeSymlink != 0 {
			link, err := fs.ReadLink(fsys, path)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			stat.Linkname = link
		}
	}

	if runtime.GOOS == "windows" {
		permPart := stat.Mode & uint32(os.ModePerm)
		noPermPart := stat.Mode &^ uint32(os.ModePerm)
		// Add the x bit: make everything +x from windows
		permPart |= 0o111
		permPart &= 0o755
		stat.Mode = noPermPart | permPart
	}

	// Clear the socket bit since archive/tar.FileInfoHeader does not handle it
	stat.Mode &^= uint32(os.ModeSocket)

	return stat, nil
}

type gatewayContainerMount struct {
	Src executor.Mountable
	FS  fs.FS
}
