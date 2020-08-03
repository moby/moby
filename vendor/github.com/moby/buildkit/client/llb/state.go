package llb

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type StateOption func(State) State

type Output interface {
	ToInput(context.Context, *Constraints) (*pb.Input, error)
	Vertex(context.Context) Vertex
}

type Vertex interface {
	Validate(context.Context) error
	Marshal(context.Context, *Constraints) (digest.Digest, []byte, *pb.OpMetadata, []*SourceLocation, error)
	Output() Output
	Inputs() []Output
}

func NewState(o Output) State {
	s := State{
		out: o,
	}.Dir("/")
	s = s.ensurePlatform()
	return s
}

type State struct {
	out   Output
	prev  *State
	key   interface{}
	value func(context.Context) (interface{}, error)
	opts  []ConstraintsOpt
	async *asyncState
}

func (s State) ensurePlatform() State {
	if o, ok := s.out.(interface {
		Platform() *specs.Platform
	}); ok {
		if p := o.Platform(); p != nil {
			s = platform(*p)(s)
		}
	}
	return s
}

func (s State) WithValue(k, v interface{}) State {
	return s.withValue(k, func(context.Context) (interface{}, error) {
		return v, nil
	})
}

func (s State) withValue(k interface{}, v func(context.Context) (interface{}, error)) State {
	return State{
		out:   s.Output(),
		prev:  &s, // doesn't need to be original pointer
		key:   k,
		value: v,
	}
}

func (s State) Value(ctx context.Context, k interface{}) (interface{}, error) {
	return s.getValue(k)(ctx)
}

func (s State) getValue(k interface{}) func(context.Context) (interface{}, error) {
	if s.key == k {
		return s.value
	}
	if s.async != nil {
		return func(ctx context.Context) (interface{}, error) {
			err := s.async.Do(ctx)
			if err != nil {
				return nil, err
			}
			return s.async.target.getValue(k)(ctx)
		}
	}
	if s.prev == nil {
		return nilValue
	}
	return s.prev.getValue(k)
}

func (s State) Async(f func(context.Context, State) (State, error)) State {
	s2 := State{
		async: &asyncState{f: f, prev: s},
	}
	return s2
}

func (s State) SetMarshalDefaults(co ...ConstraintsOpt) State {
	s.opts = co
	return s
}

func (s State) Marshal(ctx context.Context, co ...ConstraintsOpt) (*Definition, error) {
	def := &Definition{
		Metadata: make(map[digest.Digest]pb.OpMetadata, 0),
	}
	if s.Output() == nil || s.Output().Vertex(ctx) == nil {
		return def, nil
	}

	defaultPlatform := platforms.Normalize(platforms.DefaultSpec())
	c := &Constraints{
		Platform:      &defaultPlatform,
		LocalUniqueID: identity.NewID(),
	}
	for _, o := range append(s.opts, co...) {
		o.SetConstraintsOption(c)
	}

	smc := newSourceMapCollector()

	def, err := marshal(ctx, s.Output().Vertex(ctx), def, smc, map[digest.Digest]struct{}{}, map[Vertex]struct{}{}, c)
	if err != nil {
		return def, err
	}
	inp, err := s.Output().ToInput(ctx, c)
	if err != nil {
		return def, err
	}
	proto := &pb.Op{Inputs: []*pb.Input{inp}}
	dt, err := proto.Marshal()
	if err != nil {
		return def, err
	}
	def.Def = append(def.Def, dt)

	dgst := digest.FromBytes(dt)
	md := def.Metadata[dgst]
	md.Caps = map[apicaps.CapID]bool{
		pb.CapConstraints: true,
		pb.CapPlatform:    true,
	}

	for _, m := range def.Metadata {
		if m.IgnoreCache {
			md.Caps[pb.CapMetaIgnoreCache] = true
		}
		if m.Description != nil {
			md.Caps[pb.CapMetaDescription] = true
		}
		if m.ExportCache != nil {
			md.Caps[pb.CapMetaExportCache] = true
		}
	}

	def.Metadata[dgst] = md
	sm, err := smc.Marshal(ctx, co...)
	if err != nil {
		return nil, err
	}
	def.Source = sm

	return def, nil
}

func marshal(ctx context.Context, v Vertex, def *Definition, s *sourceMapCollector, cache map[digest.Digest]struct{}, vertexCache map[Vertex]struct{}, c *Constraints) (*Definition, error) {
	if _, ok := vertexCache[v]; ok {
		return def, nil
	}
	for _, inp := range v.Inputs() {
		var err error
		def, err = marshal(ctx, inp.Vertex(ctx), def, s, cache, vertexCache, c)
		if err != nil {
			return def, err
		}
	}

	dgst, dt, opMeta, sls, err := v.Marshal(ctx, c)
	if err != nil {
		return def, err
	}
	vertexCache[v] = struct{}{}
	if opMeta != nil {
		def.Metadata[dgst] = mergeMetadata(def.Metadata[dgst], *opMeta)
	}
	if _, ok := cache[dgst]; ok {
		return def, nil
	}
	s.Add(dgst, sls)
	def.Def = append(def.Def, dt)
	cache[dgst] = struct{}{}
	return def, nil
}

func (s State) Validate(ctx context.Context) error {
	return s.Output().Vertex(ctx).Validate(ctx)
}

func (s State) Output() Output {
	if s.async != nil {
		return s.async.Output()
	}
	return s.out
}

func (s State) WithOutput(o Output) State {
	prev := s
	s = State{
		out:  o,
		prev: &prev,
	}
	s = s.ensurePlatform()
	return s
}

func (s State) WithImageConfig(c []byte) (State, error) {
	var img struct {
		Config struct {
			Env        []string `json:"Env,omitempty"`
			WorkingDir string   `json:"WorkingDir,omitempty"`
			User       string   `json:"User,omitempty"`
		} `json:"config,omitempty"`
	}
	if err := json.Unmarshal(c, &img); err != nil {
		return State{}, err
	}
	for _, env := range img.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts[0]) > 0 {
			var v string
			if len(parts) > 1 {
				v = parts[1]
			}
			s = s.AddEnv(parts[0], v)
		}
	}
	s = s.Dir(img.Config.WorkingDir)
	return s, nil
}

func (s State) Run(ro ...RunOption) ExecState {
	ei := &ExecInfo{State: s}
	for _, o := range ro {
		o.SetRunOption(ei)
	}
	exec := NewExecOp(ei.State, ei.ProxyEnv, ei.ReadonlyRootFS, ei.Constraints)
	for _, m := range ei.Mounts {
		exec.AddMount(m.Target, m.Source, m.Opts...)
	}
	exec.secrets = ei.Secrets
	exec.ssh = ei.SSH

	return ExecState{
		State: s.WithOutput(exec.Output()),
		exec:  exec,
	}
}

func (s State) File(a *FileAction, opts ...ConstraintsOpt) State {
	var c Constraints
	for _, o := range opts {
		o.SetConstraintsOption(&c)
	}

	return s.WithOutput(NewFileOp(s, a, c).Output())
}

func (s State) AddEnv(key, value string) State {
	return AddEnv(key, value)(s)
}

func (s State) AddEnvf(key, value string, v ...interface{}) State {
	return AddEnvf(key, value, v...)(s)
}

func (s State) Dir(str string) State {
	return Dir(str)(s)
}
func (s State) Dirf(str string, v ...interface{}) State {
	return Dirf(str, v...)(s)
}

func (s State) GetEnv(ctx context.Context, key string) (string, bool, error) {
	env, err := getEnv(s)(ctx)
	if err != nil {
		return "", false, err
	}
	v, ok := env.Get(key)
	return v, ok, nil
}

func (s State) Env(ctx context.Context) ([]string, error) {
	env, err := getEnv(s)(ctx)
	if err != nil {
		return nil, err
	}
	return env.ToArray(), nil
}

func (s State) GetDir(ctx context.Context) (string, error) {
	return getDir(s)(ctx)
}

func (s State) GetArgs(ctx context.Context) ([]string, error) {
	return getArgs(s)(ctx)
}

func (s State) Reset(s2 State) State {
	return Reset(s2)(s)
}

func (s State) User(v string) State {
	return User(v)(s)
}

func (s State) Platform(p specs.Platform) State {
	return platform(p)(s)
}

func (s State) GetPlatform(ctx context.Context) (*specs.Platform, error) {
	return getPlatform(s)(ctx)
}

func (s State) Network(n pb.NetMode) State {
	return Network(n)(s)
}

func (s State) GetNetwork(ctx context.Context) (pb.NetMode, error) {
	return getNetwork(s)(ctx)
}
func (s State) Security(n pb.SecurityMode) State {
	return Security(n)(s)
}

func (s State) GetSecurity(ctx context.Context) (pb.SecurityMode, error) {
	return getSecurity(s)(ctx)
}

func (s State) With(so ...StateOption) State {
	for _, o := range so {
		s = o(s)
	}
	return s
}

func (s State) AddExtraHost(host string, ip net.IP) State {
	return extraHost(host, ip)(s)
}

func (s State) isFileOpCopyInput() {}

type output struct {
	vertex   Vertex
	getIndex func() (pb.OutputIndex, error)
	err      error
	platform *specs.Platform
}

func (o *output) ToInput(ctx context.Context, c *Constraints) (*pb.Input, error) {
	if o.err != nil {
		return nil, o.err
	}
	var index pb.OutputIndex
	if o.getIndex != nil {
		var err error
		index, err = o.getIndex()
		if err != nil {
			return nil, err
		}
	}
	dgst, _, _, _, err := o.vertex.Marshal(ctx, c)
	if err != nil {
		return nil, err
	}
	return &pb.Input{Digest: dgst, Index: index}, nil
}

func (o *output) Vertex(context.Context) Vertex {
	return o.vertex
}

func (o *output) Platform() *specs.Platform {
	return o.platform
}

type ConstraintsOpt interface {
	SetConstraintsOption(*Constraints)
	RunOption
	LocalOption
	HTTPOption
	ImageOption
	GitOption
}

type constraintsOptFunc func(m *Constraints)

func (fn constraintsOptFunc) SetConstraintsOption(m *Constraints) {
	fn(m)
}

func (fn constraintsOptFunc) SetRunOption(ei *ExecInfo) {
	ei.applyConstraints(fn)
}

func (fn constraintsOptFunc) SetLocalOption(li *LocalInfo) {
	li.applyConstraints(fn)
}

func (fn constraintsOptFunc) SetHTTPOption(hi *HTTPInfo) {
	hi.applyConstraints(fn)
}

func (fn constraintsOptFunc) SetImageOption(ii *ImageInfo) {
	ii.applyConstraints(fn)
}

func (fn constraintsOptFunc) SetGitOption(gi *GitInfo) {
	gi.applyConstraints(fn)
}

func mergeMetadata(m1, m2 pb.OpMetadata) pb.OpMetadata {
	if m2.IgnoreCache {
		m1.IgnoreCache = true
	}
	if len(m2.Description) > 0 {
		if m1.Description == nil {
			m1.Description = make(map[string]string)
		}
		for k, v := range m2.Description {
			m1.Description[k] = v
		}
	}
	if m2.ExportCache != nil {
		m1.ExportCache = m2.ExportCache
	}

	for k := range m2.Caps {
		if m1.Caps == nil {
			m1.Caps = make(map[apicaps.CapID]bool, len(m2.Caps))
		}
		m1.Caps[k] = true
	}

	return m1
}

var IgnoreCache = constraintsOptFunc(func(c *Constraints) {
	c.Metadata.IgnoreCache = true
})

func WithDescription(m map[string]string) ConstraintsOpt {
	return constraintsOptFunc(func(c *Constraints) {
		if c.Metadata.Description == nil {
			c.Metadata.Description = map[string]string{}
		}
		for k, v := range m {
			c.Metadata.Description[k] = v
		}
	})
}

func WithCustomName(name string) ConstraintsOpt {
	return WithDescription(map[string]string{
		"llb.customname": name,
	})
}

func WithCustomNamef(name string, a ...interface{}) ConstraintsOpt {
	return WithCustomName(fmt.Sprintf(name, a...))
}

// WithExportCache forces results for this vertex to be exported with the cache
func WithExportCache() ConstraintsOpt {
	return constraintsOptFunc(func(c *Constraints) {
		c.Metadata.ExportCache = &pb.ExportCache{Value: true}
	})
}

// WithoutExportCache sets results for this vertex to be not exported with
// the cache
func WithoutExportCache() ConstraintsOpt {
	return constraintsOptFunc(func(c *Constraints) {
		// ExportCache with value false means to disable exporting
		c.Metadata.ExportCache = &pb.ExportCache{Value: false}
	})
}

// WithoutDefaultExportCache resets the cache export for the vertex to use
// the default defined by the build configuration.
func WithoutDefaultExportCache() ConstraintsOpt {
	return constraintsOptFunc(func(c *Constraints) {
		// nil means no vertex based config has been set
		c.Metadata.ExportCache = nil
	})
}

// WithCaps exposes supported LLB caps to the marshaler
func WithCaps(caps apicaps.CapSet) ConstraintsOpt {
	return constraintsOptFunc(func(c *Constraints) {
		c.Caps = &caps
	})
}

type constraintsWrapper struct {
	Constraints
}

func (cw *constraintsWrapper) applyConstraints(f func(c *Constraints)) {
	f(&cw.Constraints)
}

type Constraints struct {
	Platform          *specs.Platform
	WorkerConstraints []string
	Metadata          pb.OpMetadata
	LocalUniqueID     string
	Caps              *apicaps.CapSet
	SourceLocations   []*SourceLocation
}

func Platform(p specs.Platform) ConstraintsOpt {
	return constraintsOptFunc(func(c *Constraints) {
		c.Platform = &p
	})
}

func LocalUniqueID(v string) ConstraintsOpt {
	return constraintsOptFunc(func(c *Constraints) {
		c.LocalUniqueID = v
	})
}

var (
	LinuxAmd64   = Platform(specs.Platform{OS: "linux", Architecture: "amd64"})
	LinuxArmhf   = Platform(specs.Platform{OS: "linux", Architecture: "arm", Variant: "v7"})
	LinuxArm     = LinuxArmhf
	LinuxArmel   = Platform(specs.Platform{OS: "linux", Architecture: "arm", Variant: "v6"})
	LinuxArm64   = Platform(specs.Platform{OS: "linux", Architecture: "arm64"})
	LinuxS390x   = Platform(specs.Platform{OS: "linux", Architecture: "s390x"})
	LinuxPpc64le = Platform(specs.Platform{OS: "linux", Architecture: "ppc64le"})
	Darwin       = Platform(specs.Platform{OS: "darwin", Architecture: "amd64"})
	Windows      = Platform(specs.Platform{OS: "windows", Architecture: "amd64"})
)

func Require(filters ...string) ConstraintsOpt {
	return constraintsOptFunc(func(c *Constraints) {
		for _, f := range filters {
			c.WorkerConstraints = append(c.WorkerConstraints, f)
		}
	})
}

func nilValue(context.Context) (interface{}, error) {
	return nil, nil
}
