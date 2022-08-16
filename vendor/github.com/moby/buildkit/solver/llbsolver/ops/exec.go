package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/frontend/gateway"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/secrets"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/llbsolver/errdefs"
	"github.com/moby/buildkit/solver/llbsolver/mounts"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/progress/controller"
	"github.com/moby/buildkit/util/progress/logs"
	utilsystem "github.com/moby/buildkit/util/system"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/semaphore"
)

const execCacheType = "buildkit.exec.v0"

type execOp struct {
	op          *pb.ExecOp
	cm          cache.Manager
	mm          *mounts.MountManager
	sm          *session.Manager
	exec        executor.Executor
	w           worker.Worker
	platform    *pb.Platform
	numInputs   int
	parallelism *semaphore.Weighted
	vtx         solver.Vertex
}

func NewExecOp(v solver.Vertex, op *pb.Op_Exec, platform *pb.Platform, cm cache.Manager, parallelism *semaphore.Weighted, sm *session.Manager, exec executor.Executor, w worker.Worker) (solver.Op, error) {
	if err := llbsolver.ValidateOp(&pb.Op{Op: op}); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("exec %s", strings.Join(op.Exec.Meta.Args, " "))
	return &execOp{
		op:          op.Exec,
		mm:          mounts.NewMountManager(name, cm, sm),
		cm:          cm,
		sm:          sm,
		exec:        exec,
		numInputs:   len(v.Inputs()),
		w:           w,
		platform:    platform,
		parallelism: parallelism,
		vtx:         v,
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
	for i := range old.Mounts {
		m := *old.Mounts[i]
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
		p = ocispecs.Platform{
			OS:           e.platform.OS,
			Architecture: e.platform.Architecture,
			Variant:      e.platform.Variant,
		}
	}

	// Special case for cache compatibility with buggy versions that wrongly
	// excluded Exec.Mounts: for the default case of one root mount (i.e. RUN
	// inside a Dockerfile), do not include the mount when generating the cache
	// map.
	if len(op.Mounts) == 1 &&
		op.Mounts[0].Dest == "/" &&
		op.Mounts[0].Selector == "" &&
		!op.Mounts[0].Readonly &&
		op.Mounts[0].MountType == pb.MountType_BIND &&
		op.Mounts[0].CacheOpt == nil &&
		op.Mounts[0].SSHOpt == nil &&
		op.Mounts[0].SecretOpt == nil &&
		op.Mounts[0].ResultID == "" {
		op.Mounts = nil
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
		Opts: solver.CacheOpts(map[interface{}]interface{}{
			cache.ProgressKey{}: &controller.Controller{
				WriterFactory: progress.FromContext(ctx),
				Digest:        e.vtx.Digest(),
				Name:          e.vtx.Name(),
				ProgressGroup: e.vtx.Options().ProgressGroup,
			},
		}),
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

func (e *execOp) Exec(ctx context.Context, g session.Group, inputs []solver.Result) (results []solver.Result, err error) {
	trace.SpanFromContext(ctx).AddEvent("ExecOp started")

	refs := make([]*worker.WorkerRef, len(inputs))
	for i, inp := range inputs {
		var ok bool
		refs[i], ok = inp.Sys().(*worker.WorkerRef)
		if !ok {
			return nil, errors.Errorf("invalid reference for exec %T", inp.Sys())
		}
	}

	p, err := gateway.PrepareMounts(ctx, e.mm, e.cm, g, e.op.Meta.Cwd, e.op.Mounts, refs, func(m *pb.Mount, ref cache.ImmutableRef) (cache.MutableRef, error) {
		desc := fmt.Sprintf("mount %s from exec %s", m.Dest, strings.Join(e.op.Meta.Args, " "))
		return e.cm.New(ctx, ref, g, cache.WithDescription(desc))
	})
	defer func() {
		if err != nil {
			execInputs := make([]solver.Result, len(e.op.Mounts))
			for i, m := range e.op.Mounts {
				if m.Input == -1 {
					continue
				}
				execInputs[i] = inputs[m.Input].Clone()
			}
			execMounts := make([]solver.Result, len(e.op.Mounts))
			copy(execMounts, execInputs)
			for i, res := range results {
				execMounts[p.OutputRefs[i].MountIndex] = res
			}
			for _, active := range p.Actives {
				if active.NoCommit {
					active.Ref.Release(context.TODO())
				} else {
					ref, cerr := active.Ref.Commit(ctx)
					if cerr != nil {
						err = errors.Wrapf(err, "error committing %s: %s", active.Ref.ID(), cerr)
						continue
					}
					execMounts[active.MountIndex] = worker.NewWorkerRefResult(ref, e.w)
				}
			}
			err = errdefs.WithExecError(err, execInputs, execMounts)
		} else {
			// Only release actives if err is nil.
			for i := len(p.Actives) - 1; i >= 0; i-- { // call in LIFO order
				p.Actives[i].Ref.Release(context.TODO())
			}
		}
		for _, o := range p.OutputRefs {
			if o.Ref != nil {
				o.Ref.Release(context.TODO())
			}
		}
	}()
	if err != nil {
		return nil, err
	}

	extraHosts, err := gateway.ParseExtraHosts(e.op.Meta.ExtraHosts)
	if err != nil {
		return nil, err
	}

	emu, err := getEmulator(ctx, e.platform, e.cm.IdentityMapping())
	if err != nil {
		return nil, err
	}
	if emu != nil {
		e.op.Meta.Args = append([]string{qemuMountName}, e.op.Meta.Args...)

		p.Mounts = append(p.Mounts, executor.Mount{
			Readonly: true,
			Src:      emu,
			Dest:     qemuMountName,
		})
	}

	meta := executor.Meta{
		Args:           e.op.Meta.Args,
		Env:            e.op.Meta.Env,
		Cwd:            e.op.Meta.Cwd,
		User:           e.op.Meta.User,
		Hostname:       e.op.Meta.Hostname,
		ReadonlyRootFS: p.ReadonlyRootFS,
		ExtraHosts:     extraHosts,
		Ulimit:         e.op.Meta.Ulimit,
		CgroupParent:   e.op.Meta.CgroupParent,
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

	secretEnv, err := e.loadSecretEnv(ctx, g)
	if err != nil {
		return nil, err
	}
	meta.Env = append(meta.Env, secretEnv...)

	stdout, stderr, flush := logs.NewLogStreams(ctx, os.Getenv("BUILDKIT_DEBUG_EXEC_OUTPUT") == "1")
	defer stdout.Close()
	defer stderr.Close()
	defer func() {
		if err != nil {
			flush()
		}
	}()

	execErr := e.exec.Run(ctx, "", p.Root, p.Mounts, executor.ProcessInfo{
		Meta:   meta,
		Stdin:  nil,
		Stdout: stdout,
		Stderr: stderr,
	}, nil)

	for i, out := range p.OutputRefs {
		if mutable, ok := out.Ref.(cache.MutableRef); ok {
			ref, err := mutable.Commit(ctx)
			if err != nil {
				return nil, errors.Wrapf(err, "error committing %s", mutable.ID())
			}
			results = append(results, worker.NewWorkerRefResult(ref, e.w))
		} else {
			results = append(results, worker.NewWorkerRefResult(out.Ref.(cache.ImmutableRef), e.w))
		}
		// Prevent the result from being released.
		p.OutputRefs[i].Ref = nil
	}
	return results, errors.Wrapf(execErr, "process %q did not complete successfully", strings.Join(e.op.Meta.Args, " "))
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
	if v := p.AllProxy; v != "" {
		out = append(out, "ALL_PROXY="+v, "all_proxy="+v)
	}
	return out
}

func (e *execOp) Acquire(ctx context.Context) (solver.ReleaseFunc, error) {
	if e.parallelism == nil {
		return func() {}, nil
	}
	err := e.parallelism.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	return func() {
		e.parallelism.Release(1)
	}, nil
}

func (e *execOp) loadSecretEnv(ctx context.Context, g session.Group) ([]string, error) {
	secretenv := e.op.Secretenv
	if len(secretenv) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(secretenv))
	for _, sopt := range secretenv {
		id := sopt.ID
		if id == "" {
			return nil, errors.Errorf("secret ID missing for %q environment variable", sopt.Name)
		}
		var dt []byte
		var err error
		err = e.sm.Any(ctx, g, func(ctx context.Context, _ string, caller session.Caller) error {
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
