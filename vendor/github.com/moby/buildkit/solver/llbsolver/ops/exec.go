package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/llbsolver/mounts"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/progress/logs"
	utilsystem "github.com/moby/buildkit/util/system"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const execCacheType = "buildkit.exec.v0"

type execOp struct {
	op        *pb.ExecOp
	cm        cache.Manager
	mm        *mounts.MountManager
	exec      executor.Executor
	w         worker.Worker
	platform  *pb.Platform
	numInputs int
}

func NewExecOp(v solver.Vertex, op *pb.Op_Exec, platform *pb.Platform, cm cache.Manager, sm *session.Manager, md *metadata.Store, exec executor.Executor, w worker.Worker) (solver.Op, error) {
	if err := llbsolver.ValidateOp(&pb.Op{Op: op}); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("exec %s", strings.Join(op.Exec.Meta.Args, " "))
	return &execOp{
		op:        op.Exec,
		mm:        mounts.NewMountManager(name, cm, sm, md),
		cm:        cm,
		exec:      exec,
		numInputs: len(v.Inputs()),
		w:         w,
		platform:  platform,
	}, nil
}

func cloneExecOp(old *pb.ExecOp) pb.ExecOp {
	n := *old
	meta := *n.Meta
	meta.ExtraHosts = nil
	for i := range n.Meta.ExtraHosts {
		h := *n.Meta.ExtraHosts[i]
		meta.ExtraHosts = append(meta.ExtraHosts, &h)
	}
	n.Meta = &meta
	n.Mounts = nil
	for i := range n.Mounts {
		m := *n.Mounts[i]
		n.Mounts = append(n.Mounts, &m)
	}
	return n
}

func (e *execOp) CacheMap(ctx context.Context, g session.Group, index int) (*solver.CacheMap, bool, error) {
	op := cloneExecOp(e.op)
	for i := range op.Meta.ExtraHosts {
		h := op.Meta.ExtraHosts[i]
		h.IP = ""
		op.Meta.ExtraHosts[i] = h
	}
	for i := range op.Mounts {
		op.Mounts[i].Selector = ""
	}
	op.Meta.ProxyEnv = nil

	p := platforms.DefaultSpec()
	if e.platform != nil {
		p = specs.Platform{
			OS:           e.platform.OS,
			Architecture: e.platform.Architecture,
			Variant:      e.platform.Variant,
		}
	}

	dt, err := json.Marshal(struct {
		Type    string
		Exec    *pb.ExecOp
		OS      string
		Arch    string
		Variant string `json:",omitempty"`
	}{
		Type:    execCacheType,
		Exec:    &op,
		OS:      p.OS,
		Arch:    p.Architecture,
		Variant: p.Variant,
	})
	if err != nil {
		return nil, false, err
	}

	cm := &solver.CacheMap{
		Digest: digest.FromBytes(dt),
		Deps: make([]struct {
			Selector          digest.Digest
			ComputeDigestFunc solver.ResultBasedCacheFunc
			PreprocessFunc    solver.PreprocessFunc
		}, e.numInputs),
	}

	deps, err := e.getMountDeps()
	if err != nil {
		return nil, false, err
	}

	for i, dep := range deps {
		if len(dep.Selectors) != 0 {
			dgsts := make([][]byte, 0, len(dep.Selectors))
			for _, p := range dep.Selectors {
				dgsts = append(dgsts, []byte(p))
			}
			cm.Deps[i].Selector = digest.FromBytes(bytes.Join(dgsts, []byte{0}))
		}
		if !dep.NoContentBasedHash {
			cm.Deps[i].ComputeDigestFunc = llbsolver.NewContentHashFunc(toSelectors(dedupePaths(dep.Selectors)))
		}
		cm.Deps[i].PreprocessFunc = llbsolver.UnlazyResultFunc
	}

	return cm, true, nil
}

func dedupePaths(inp []string) []string {
	old := make(map[string]struct{}, len(inp))
	for _, p := range inp {
		old[p] = struct{}{}
	}
	paths := make([]string, 0, len(old))
	for p1 := range old {
		var skip bool
		for p2 := range old {
			if p1 != p2 && strings.HasPrefix(p1, p2+"/") {
				skip = true
				break
			}
		}
		if !skip {
			paths = append(paths, p1)
		}
	}
	sort.Slice(paths, func(i, j int) bool {
		return paths[i] < paths[j]
	})
	return paths
}

func toSelectors(p []string) []llbsolver.Selector {
	sel := make([]llbsolver.Selector, 0, len(p))
	for _, p := range p {
		sel = append(sel, llbsolver.Selector{Path: p, FollowLinks: true})
	}
	return sel
}

type dep struct {
	Selectors          []string
	NoContentBasedHash bool
}

func (e *execOp) getMountDeps() ([]dep, error) {
	deps := make([]dep, e.numInputs)
	for _, m := range e.op.Mounts {
		if m.Input == pb.Empty {
			continue
		}
		if int(m.Input) >= len(deps) {
			return nil, errors.Errorf("invalid mountinput %v", m)
		}

		sel := m.Selector
		if sel != "" {
			sel = path.Join("/", sel)
			deps[m.Input].Selectors = append(deps[m.Input].Selectors, sel)
		}

		if (!m.Readonly || m.Dest == pb.RootMount) && m.Output != -1 { // exclude read-only rootfs && read-write mounts
			deps[m.Input].NoContentBasedHash = true
		}
	}
	return deps, nil
}

func addDefaultEnvvar(env []string, k, v string) []string {
	for _, e := range env {
		if strings.HasPrefix(e, k+"=") {
			return env
		}
	}
	return append(env, k+"="+v)
}

func (e *execOp) Exec(ctx context.Context, g session.Group, inputs []solver.Result) ([]solver.Result, error) {
	var mounts []executor.Mount
	var root cache.Mountable
	var readonlyRootFS bool

	var outputs []cache.Ref

	defer func() {
		for _, o := range outputs {
			if o != nil {
				go o.Release(context.TODO())
			}
		}
	}()

	// loop over all mounts, fill in mounts, root and outputs
	for _, m := range e.op.Mounts {
		var mountable cache.Mountable
		var ref cache.ImmutableRef

		if m.Dest == pb.RootMount && m.MountType != pb.MountType_BIND {
			return nil, errors.Errorf("invalid mount type %s for %s", m.MountType.String(), m.Dest)
		}

		// if mount is based on input validate and load it
		if m.Input != pb.Empty {
			if int(m.Input) > len(inputs) {
				return nil, errors.Errorf("missing input %d", m.Input)
			}
			inp := inputs[int(m.Input)]
			workerRef, ok := inp.Sys().(*worker.WorkerRef)
			if !ok {
				return nil, errors.Errorf("invalid reference for exec %T", inp.Sys())
			}
			ref = workerRef.ImmutableRef
			mountable = ref
		}

		makeMutable := func(ref cache.ImmutableRef) (cache.MutableRef, error) {
			desc := fmt.Sprintf("mount %s from exec %s", m.Dest, strings.Join(e.op.Meta.Args, " "))
			return e.cm.New(ctx, ref, g, cache.WithDescription(desc))
		}

		switch m.MountType {
		case pb.MountType_BIND:
			// if mount creates an output
			if m.Output != pb.SkipOutput {
				// if it is readonly and not root then output is the input
				if m.Readonly && ref != nil && m.Dest != pb.RootMount {
					outputs = append(outputs, ref.Clone())
				} else {
					// otherwise output and mount is the mutable child
					active, err := makeMutable(ref)
					if err != nil {
						return nil, err
					}
					outputs = append(outputs, active)
					mountable = active
				}
			} else if (!m.Readonly || ref == nil) && m.Dest != pb.RootMount {
				// this case is empty readonly scratch without output that is not really useful for anything but don't error
				active, err := makeMutable(ref)
				if err != nil {
					return nil, err
				}
				defer active.Release(context.TODO())
				mountable = active
			}

		case pb.MountType_CACHE:
			mRef, err := e.mm.MountableCache(ctx, m, ref, g)
			if err != nil {
				return nil, err
			}
			mountable = mRef
			defer func() {
				go mRef.Release(context.TODO())
			}()
			if m.Output != pb.SkipOutput && ref != nil {
				outputs = append(outputs, ref.Clone())
			}

		case pb.MountType_TMPFS:
			mountable = e.mm.MountableTmpFS()
		case pb.MountType_SECRET:
			var err error
			mountable, err = e.mm.MountableSecret(ctx, m, g)
			if err != nil {
				return nil, err
			}
			if mountable == nil {
				continue
			}
		case pb.MountType_SSH:
			var err error
			mountable, err = e.mm.MountableSSH(ctx, m, g)
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

		// if dest is root we need mutable ref even if there is no output
		if m.Dest == pb.RootMount {
			root = mountable
			readonlyRootFS = m.Readonly
			if m.Output == pb.SkipOutput && readonlyRootFS {
				active, err := makeMutable(ref)
				if err != nil {
					return nil, err
				}
				defer func() {
					go active.Release(context.TODO())
				}()
				root = active
			}
		} else {
			mws := mountWithSession(mountable, g)
			mws.Dest = m.Dest
			mws.Readonly = m.Readonly
			mws.Selector = m.Selector
			mounts = append(mounts, mws)
		}
	}

	// sort mounts so parents are mounted first
	sort.Slice(mounts, func(i, j int) bool {
		return mounts[i].Dest < mounts[j].Dest
	})

	extraHosts, err := parseExtraHosts(e.op.Meta.ExtraHosts)
	if err != nil {
		return nil, err
	}

	emu, err := getEmulator(e.platform, e.cm.IdentityMapping())
	if err == nil && emu != nil {
		e.op.Meta.Args = append([]string{qemuMountName}, e.op.Meta.Args...)

		mounts = append(mounts, executor.Mount{
			Readonly: true,
			Src:      emu,
			Dest:     qemuMountName,
		})
	}
	if err != nil {
		logrus.Warn(err.Error()) // TODO: remove this with pull support
	}

	meta := executor.Meta{
		Args:           e.op.Meta.Args,
		Env:            e.op.Meta.Env,
		Cwd:            e.op.Meta.Cwd,
		User:           e.op.Meta.User,
		Hostname:       e.op.Meta.Hostname,
		ReadonlyRootFS: readonlyRootFS,
		ExtraHosts:     extraHosts,
		NetMode:        e.op.Network,
		SecurityMode:   e.op.Security,
	}

	if e.op.Meta.ProxyEnv != nil {
		meta.Env = append(meta.Env, proxyEnvList(e.op.Meta.ProxyEnv)...)
	}
	var currentOS string
	if e.platform != nil {
		currentOS = e.platform.OS
	}
	meta.Env = addDefaultEnvvar(meta.Env, "PATH", utilsystem.DefaultPathEnv(currentOS))

	stdout, stderr := logs.NewLogStreams(ctx, os.Getenv("BUILDKIT_DEBUG_EXEC_OUTPUT") == "1")
	defer stdout.Close()
	defer stderr.Close()

	if err := e.exec.Run(ctx, "", mountWithSession(root, g), mounts, executor.ProcessInfo{Meta: meta, Stdin: nil, Stdout: stdout, Stderr: stderr}, nil); err != nil {
		return nil, errors.Wrapf(err, "executor failed running %v", meta.Args)
	}

	refs := []solver.Result{}
	for i, out := range outputs {
		if mutable, ok := out.(cache.MutableRef); ok {
			ref, err := mutable.Commit(ctx)
			if err != nil {
				return nil, errors.Wrapf(err, "error committing %s", mutable.ID())
			}
			refs = append(refs, worker.NewWorkerRefResult(ref, e.w))
		} else {
			refs = append(refs, worker.NewWorkerRefResult(out.(cache.ImmutableRef), e.w))
		}
		outputs[i] = nil
	}
	return refs, nil
}

func proxyEnvList(p *pb.ProxyEnv) []string {
	out := []string{}
	if v := p.HttpProxy; v != "" {
		out = append(out, "HTTP_PROXY="+v, "http_proxy="+v)
	}
	if v := p.HttpsProxy; v != "" {
		out = append(out, "HTTPS_PROXY="+v, "https_proxy="+v)
	}
	if v := p.FtpProxy; v != "" {
		out = append(out, "FTP_PROXY="+v, "ftp_proxy="+v)
	}
	if v := p.NoProxy; v != "" {
		out = append(out, "NO_PROXY="+v, "no_proxy="+v)
	}
	return out
}

func parseExtraHosts(ips []*pb.HostIP) ([]executor.HostIP, error) {
	out := make([]executor.HostIP, len(ips))
	for i, hip := range ips {
		ip := net.ParseIP(hip.IP)
		if ip == nil {
			return nil, errors.Errorf("failed to parse IP %s", hip.IP)
		}
		out[i] = executor.HostIP{
			IP:   ip,
			Host: hip.Host,
		}
	}
	return out, nil
}

func mountWithSession(m cache.Mountable, g session.Group) executor.Mount {
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
