package llb

import (
	"context"
	_ "crypto/sha256" // for opencontainers/go-digest
	"fmt"
	"net"
	"sort"

	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/system"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func NewExecOp(base State, proxyEnv *ProxyEnv, readOnly bool, c Constraints) *ExecOp {
	e := &ExecOp{base: base, constraints: c, proxyEnv: proxyEnv}
	root := base.Output()
	rootMount := &mount{
		target:   pb.RootMount,
		source:   root,
		readonly: readOnly,
	}
	e.mounts = append(e.mounts, rootMount)
	if readOnly {
		e.root = root
	} else {
		o := &output{vertex: e, getIndex: e.getMountIndexFn(rootMount)}
		if p := c.Platform; p != nil {
			o.platform = p
		}
		e.root = o
	}
	rootMount.output = e.root
	return e
}

type mount struct {
	target       string
	readonly     bool
	source       Output
	output       Output
	selector     string
	cacheID      string
	tmpfs        bool
	cacheSharing CacheMountSharingMode
	noOutput     bool
}

type ExecOp struct {
	MarshalCache
	proxyEnv    *ProxyEnv
	root        Output
	mounts      []*mount
	base        State
	constraints Constraints
	isValidated bool
	secrets     []SecretInfo
	ssh         []SSHInfo
}

func (e *ExecOp) AddMount(target string, source Output, opt ...MountOption) Output {
	m := &mount{
		target: target,
		source: source,
	}
	for _, o := range opt {
		o(m)
	}
	e.mounts = append(e.mounts, m)
	if m.readonly {
		m.output = source
	} else if m.tmpfs {
		m.output = &output{vertex: e, err: errors.Errorf("tmpfs mount for %s can't be used as a parent", target)}
	} else if m.noOutput {
		m.output = &output{vertex: e, err: errors.Errorf("mount marked no-output and %s can't be used as a parent", target)}
	} else {
		o := &output{vertex: e, getIndex: e.getMountIndexFn(m)}
		if p := e.constraints.Platform; p != nil {
			o.platform = p
		}
		m.output = o
	}
	e.Store(nil, nil, nil, nil)
	e.isValidated = false
	return m.output
}

func (e *ExecOp) GetMount(target string) Output {
	for _, m := range e.mounts {
		if m.target == target {
			return m.output
		}
	}
	return nil
}

func (e *ExecOp) Validate(ctx context.Context, c *Constraints) error {
	if e.isValidated {
		return nil
	}
	args, err := getArgs(e.base)(ctx, c)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.Errorf("arguments are required")
	}
	cwd, err := getDir(e.base)(ctx, c)
	if err != nil {
		return err
	}
	if cwd == "" {
		return errors.Errorf("working directory is required")
	}
	for _, m := range e.mounts {
		if m.source != nil {
			if err := m.source.Vertex(ctx, c).Validate(ctx, c); err != nil {
				return err
			}
		}
	}
	e.isValidated = true
	return nil
}

func (e *ExecOp) Marshal(ctx context.Context, c *Constraints) (digest.Digest, []byte, *pb.OpMetadata, []*SourceLocation, error) {
	if e.Cached(c) {
		return e.Load()
	}
	if err := e.Validate(ctx, c); err != nil {
		return "", nil, nil, nil, err
	}
	// make sure mounts are sorted
	sort.Slice(e.mounts, func(i, j int) bool {
		return e.mounts[i].target < e.mounts[j].target
	})

	env, err := getEnv(e.base)(ctx, c)
	if err != nil {
		return "", nil, nil, nil, err
	}

	if len(e.ssh) > 0 {
		for i, s := range e.ssh {
			if s.Target == "" {
				e.ssh[i].Target = fmt.Sprintf("/run/buildkit/ssh_agent.%d", i)
			}
		}
		if _, ok := env.Get("SSH_AUTH_SOCK"); !ok {
			env = env.AddOrReplace("SSH_AUTH_SOCK", e.ssh[0].Target)
		}
	}
	if c.Caps != nil {
		if err := c.Caps.Supports(pb.CapExecMetaSetsDefaultPath); err != nil {
			os := "linux"
			if c.Platform != nil {
				os = c.Platform.OS
			} else if e.constraints.Platform != nil {
				os = e.constraints.Platform.OS
			}
			env = env.SetDefault("PATH", system.DefaultPathEnv(os))
		} else {
			addCap(&e.constraints, pb.CapExecMetaSetsDefaultPath)
		}
	}

	args, err := getArgs(e.base)(ctx, c)
	if err != nil {
		return "", nil, nil, nil, err
	}

	cwd, err := getDir(e.base)(ctx, c)
	if err != nil {
		return "", nil, nil, nil, err
	}

	user, err := getUser(e.base)(ctx, c)
	if err != nil {
		return "", nil, nil, nil, err
	}

	hostname, err := getHostname(e.base)(ctx, c)
	if err != nil {
		return "", nil, nil, nil, err
	}

	meta := &pb.Meta{
		Args:     args,
		Env:      env.ToArray(),
		Cwd:      cwd,
		User:     user,
		Hostname: hostname,
	}
	extraHosts, err := getExtraHosts(e.base)(ctx, c)
	if err != nil {
		return "", nil, nil, nil, err
	}
	if len(extraHosts) > 0 {
		hosts := make([]*pb.HostIP, len(extraHosts))
		for i, h := range extraHosts {
			hosts[i] = &pb.HostIP{Host: h.Host, IP: h.IP.String()}
		}
		meta.ExtraHosts = hosts
	}

	network, err := getNetwork(e.base)(ctx, c)
	if err != nil {
		return "", nil, nil, nil, err
	}

	security, err := getSecurity(e.base)(ctx, c)
	if err != nil {
		return "", nil, nil, nil, err
	}

	peo := &pb.ExecOp{
		Meta:     meta,
		Network:  network,
		Security: security,
	}
	if network != NetModeSandbox {
		addCap(&e.constraints, pb.CapExecMetaNetwork)
	}

	if security != SecurityModeSandbox {
		addCap(&e.constraints, pb.CapExecMetaSecurity)
	}

	if p := e.proxyEnv; p != nil {
		peo.Meta.ProxyEnv = &pb.ProxyEnv{
			HttpProxy:  p.HTTPProxy,
			HttpsProxy: p.HTTPSProxy,
			FtpProxy:   p.FTPProxy,
			NoProxy:    p.NoProxy,
			AllProxy:   p.AllProxy,
		}
		addCap(&e.constraints, pb.CapExecMetaProxy)
	}

	addCap(&e.constraints, pb.CapExecMetaBase)

	for _, m := range e.mounts {
		if m.selector != "" {
			addCap(&e.constraints, pb.CapExecMountSelector)
		}
		if m.cacheID != "" {
			addCap(&e.constraints, pb.CapExecMountCache)
			addCap(&e.constraints, pb.CapExecMountCacheSharing)
		} else if m.tmpfs {
			addCap(&e.constraints, pb.CapExecMountTmpfs)
		} else if m.source != nil {
			addCap(&e.constraints, pb.CapExecMountBind)
		}
	}

	if len(e.secrets) > 0 {
		addCap(&e.constraints, pb.CapExecMountSecret)
	}

	if len(e.ssh) > 0 {
		addCap(&e.constraints, pb.CapExecMountSSH)
	}

	if e.constraints.Platform == nil {
		p, err := getPlatform(e.base)(ctx, c)
		if err != nil {
			return "", nil, nil, nil, err
		}
		e.constraints.Platform = p
	}

	pop, md := MarshalConstraints(c, &e.constraints)
	pop.Op = &pb.Op_Exec{
		Exec: peo,
	}

	outIndex := 0
	for _, m := range e.mounts {
		inputIndex := pb.InputIndex(len(pop.Inputs))
		if m.source != nil {
			if m.tmpfs {
				return "", nil, nil, nil, errors.Errorf("tmpfs mounts must use scratch")
			}
			inp, err := m.source.ToInput(ctx, c)
			if err != nil {
				return "", nil, nil, nil, err
			}

			newInput := true

			for i, inp2 := range pop.Inputs {
				if *inp == *inp2 {
					inputIndex = pb.InputIndex(i)
					newInput = false
					break
				}
			}

			if newInput {
				pop.Inputs = append(pop.Inputs, inp)
			}
		} else {
			inputIndex = pb.Empty
		}

		outputIndex := pb.OutputIndex(-1)
		if !m.noOutput && !m.readonly && m.cacheID == "" && !m.tmpfs {
			outputIndex = pb.OutputIndex(outIndex)
			outIndex++
		}

		pm := &pb.Mount{
			Input:    inputIndex,
			Dest:     m.target,
			Readonly: m.readonly,
			Output:   outputIndex,
			Selector: m.selector,
		}
		if m.cacheID != "" {
			pm.MountType = pb.MountType_CACHE
			pm.CacheOpt = &pb.CacheOpt{
				ID: m.cacheID,
			}
			switch m.cacheSharing {
			case CacheMountShared:
				pm.CacheOpt.Sharing = pb.CacheSharingOpt_SHARED
			case CacheMountPrivate:
				pm.CacheOpt.Sharing = pb.CacheSharingOpt_PRIVATE
			case CacheMountLocked:
				pm.CacheOpt.Sharing = pb.CacheSharingOpt_LOCKED
			}
		}
		if m.tmpfs {
			pm.MountType = pb.MountType_TMPFS
		}
		peo.Mounts = append(peo.Mounts, pm)
	}

	for _, s := range e.secrets {
		pm := &pb.Mount{
			Dest:      s.Target,
			MountType: pb.MountType_SECRET,
			SecretOpt: &pb.SecretOpt{
				ID:       s.ID,
				Uid:      uint32(s.UID),
				Gid:      uint32(s.GID),
				Optional: s.Optional,
				Mode:     uint32(s.Mode),
			},
		}
		peo.Mounts = append(peo.Mounts, pm)
	}

	for _, s := range e.ssh {
		pm := &pb.Mount{
			Dest:      s.Target,
			MountType: pb.MountType_SSH,
			SSHOpt: &pb.SSHOpt{
				ID:       s.ID,
				Uid:      uint32(s.UID),
				Gid:      uint32(s.GID),
				Mode:     uint32(s.Mode),
				Optional: s.Optional,
			},
		}
		peo.Mounts = append(peo.Mounts, pm)
	}

	dt, err := pop.Marshal()
	if err != nil {
		return "", nil, nil, nil, err
	}
	e.Store(dt, md, e.constraints.SourceLocations, c)
	return e.Load()
}

func (e *ExecOp) Output() Output {
	return e.root
}

func (e *ExecOp) Inputs() (inputs []Output) {
	mm := map[Output]struct{}{}
	for _, m := range e.mounts {
		if m.source != nil {
			mm[m.source] = struct{}{}
		}
	}
	for o := range mm {
		inputs = append(inputs, o)
	}
	return
}

func (e *ExecOp) getMountIndexFn(m *mount) func() (pb.OutputIndex, error) {
	return func() (pb.OutputIndex, error) {
		// make sure mounts are sorted
		sort.Slice(e.mounts, func(i, j int) bool {
			return e.mounts[i].target < e.mounts[j].target
		})

		i := 0
		for _, m2 := range e.mounts {
			if m2.noOutput || m2.readonly || m2.tmpfs || m2.cacheID != "" {
				continue
			}
			if m == m2 {
				return pb.OutputIndex(i), nil
			}
			i++
		}
		return pb.OutputIndex(0), errors.Errorf("invalid mount: %s", m.target)
	}
}

type ExecState struct {
	State
	exec *ExecOp
}

func (e ExecState) AddMount(target string, source State, opt ...MountOption) State {
	return source.WithOutput(e.exec.AddMount(target, source.Output(), opt...))
}

func (e ExecState) GetMount(target string) State {
	return NewState(e.exec.GetMount(target))
}

func (e ExecState) Root() State {
	return e.State
}

type MountOption func(*mount)

func Readonly(m *mount) {
	m.readonly = true
}

func SourcePath(src string) MountOption {
	return func(m *mount) {
		m.selector = src
	}
}

func ForceNoOutput(m *mount) {
	m.noOutput = true
}

func AsPersistentCacheDir(id string, sharing CacheMountSharingMode) MountOption {
	return func(m *mount) {
		m.cacheID = id
		m.cacheSharing = sharing
	}
}

func Tmpfs() MountOption {
	return func(m *mount) {
		m.tmpfs = true
	}
}

type RunOption interface {
	SetRunOption(es *ExecInfo)
}

type runOptionFunc func(*ExecInfo)

func (fn runOptionFunc) SetRunOption(ei *ExecInfo) {
	fn(ei)
}

func (fn StateOption) SetRunOption(ei *ExecInfo) {
	ei.State = ei.State.With(fn)
}

var _ RunOption = StateOption(func(_ State) State { return State{} })

func Shlex(str string) RunOption {
	return runOptionFunc(func(ei *ExecInfo) {
		ei.State = shlexf(str, false)(ei.State)
	})
}
func Shlexf(str string, v ...interface{}) RunOption {
	return runOptionFunc(func(ei *ExecInfo) {
		ei.State = shlexf(str, true, v...)(ei.State)
	})
}

func Args(a []string) RunOption {
	return runOptionFunc(func(ei *ExecInfo) {
		ei.State = args(a...)(ei.State)
	})
}

func AddExtraHost(host string, ip net.IP) RunOption {
	return runOptionFunc(func(ei *ExecInfo) {
		ei.State = ei.State.AddExtraHost(host, ip)
	})
}

func With(so ...StateOption) RunOption {
	return runOptionFunc(func(ei *ExecInfo) {
		ei.State = ei.State.With(so...)
	})
}

func AddMount(dest string, mountState State, opts ...MountOption) RunOption {
	return runOptionFunc(func(ei *ExecInfo) {
		ei.Mounts = append(ei.Mounts, MountInfo{dest, mountState.Output(), opts})
	})
}

func AddSSHSocket(opts ...SSHOption) RunOption {
	return runOptionFunc(func(ei *ExecInfo) {
		s := &SSHInfo{
			Mode: 0600,
		}
		for _, opt := range opts {
			opt.SetSSHOption(s)
		}
		ei.SSH = append(ei.SSH, *s)
	})
}

type SSHOption interface {
	SetSSHOption(*SSHInfo)
}

type sshOptionFunc func(*SSHInfo)

func (fn sshOptionFunc) SetSSHOption(si *SSHInfo) {
	fn(si)
}

func SSHID(id string) SSHOption {
	return sshOptionFunc(func(si *SSHInfo) {
		si.ID = id
	})
}

func SSHSocketTarget(target string) SSHOption {
	return sshOptionFunc(func(si *SSHInfo) {
		si.Target = target
	})
}

func SSHSocketOpt(target string, uid, gid, mode int) SSHOption {
	return sshOptionFunc(func(si *SSHInfo) {
		si.Target = target
		si.UID = uid
		si.GID = gid
		si.Mode = mode
	})
}

var SSHOptional = sshOptionFunc(func(si *SSHInfo) {
	si.Optional = true
})

type SSHInfo struct {
	ID       string
	Target   string
	Mode     int
	UID      int
	GID      int
	Optional bool
}

func AddSecret(dest string, opts ...SecretOption) RunOption {
	return runOptionFunc(func(ei *ExecInfo) {
		s := &SecretInfo{ID: dest, Target: dest, Mode: 0400}
		for _, opt := range opts {
			opt.SetSecretOption(s)
		}
		ei.Secrets = append(ei.Secrets, *s)
	})
}

type SecretOption interface {
	SetSecretOption(*SecretInfo)
}

type secretOptionFunc func(*SecretInfo)

func (fn secretOptionFunc) SetSecretOption(si *SecretInfo) {
	fn(si)
}

type SecretInfo struct {
	ID       string
	Target   string
	Mode     int
	UID      int
	GID      int
	Optional bool
}

var SecretOptional = secretOptionFunc(func(si *SecretInfo) {
	si.Optional = true
})

func SecretID(id string) SecretOption {
	return secretOptionFunc(func(si *SecretInfo) {
		si.ID = id
	})
}

func SecretFileOpt(uid, gid, mode int) SecretOption {
	return secretOptionFunc(func(si *SecretInfo) {
		si.UID = uid
		si.GID = gid
		si.Mode = mode
	})
}

func ReadonlyRootFS() RunOption {
	return runOptionFunc(func(ei *ExecInfo) {
		ei.ReadonlyRootFS = true
	})
}

func WithProxy(ps ProxyEnv) RunOption {
	return runOptionFunc(func(ei *ExecInfo) {
		ei.ProxyEnv = &ps
	})
}

type ExecInfo struct {
	constraintsWrapper
	State          State
	Mounts         []MountInfo
	ReadonlyRootFS bool
	ProxyEnv       *ProxyEnv
	Secrets        []SecretInfo
	SSH            []SSHInfo
}

type MountInfo struct {
	Target string
	Source Output
	Opts   []MountOption
}

type ProxyEnv struct {
	HTTPProxy  string
	HTTPSProxy string
	FTPProxy   string
	NoProxy    string
	AllProxy   string
}

type CacheMountSharingMode int

const (
	CacheMountShared CacheMountSharingMode = iota
	CacheMountPrivate
	CacheMountLocked
)

const (
	NetModeSandbox = pb.NetMode_UNSET
	NetModeHost    = pb.NetMode_HOST
	NetModeNone    = pb.NetMode_NONE
)

const (
	SecurityModeInsecure = pb.SecurityMode_INSECURE
	SecurityModeSandbox  = pb.SecurityMode_SANDBOX
)
