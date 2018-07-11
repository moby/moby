package llb

import (
	"context"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/system"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type StateOption func(State) State

type Output interface {
	ToInput(*Constraints) (*pb.Input, error)
	Vertex() Vertex
}

type Vertex interface {
	Validate() error
	Marshal(*Constraints) (digest.Digest, []byte, *pb.OpMetadata, error)
	Output() Output
	Inputs() []Output
}

func NewState(o Output) State {
	s := State{
		out: o,
		ctx: context.Background(),
	}
	s = dir("/")(s)
	s = addEnv("PATH", system.DefaultPathEnv)(s)
	s = s.ensurePlatform()
	return s
}

type State struct {
	out  Output
	ctx  context.Context
	opts []ConstraintsOpt
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
	return State{
		out: s.out,
		ctx: context.WithValue(s.ctx, k, v),
	}
}

func (s State) Value(k interface{}) interface{} {
	return s.ctx.Value(k)
}

func (s State) SetMarhalDefaults(co ...ConstraintsOpt) State {
	s.opts = co
	return s
}

func (s State) Marshal(co ...ConstraintsOpt) (*Definition, error) {
	def := &Definition{
		Metadata: make(map[digest.Digest]pb.OpMetadata, 0),
	}
	if s.Output() == nil {
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

	def, err := marshal(s.Output().Vertex(), def, map[digest.Digest]struct{}{}, map[Vertex]struct{}{}, c)
	if err != nil {
		return def, err
	}
	inp, err := s.Output().ToInput(c)
	if err != nil {
		return def, err
	}
	proto := &pb.Op{Inputs: []*pb.Input{inp}}
	dt, err := proto.Marshal()
	if err != nil {
		return def, err
	}
	def.Def = append(def.Def, dt)
	return def, nil
}

func marshal(v Vertex, def *Definition, cache map[digest.Digest]struct{}, vertexCache map[Vertex]struct{}, c *Constraints) (*Definition, error) {
	if _, ok := vertexCache[v]; ok {
		return def, nil
	}
	for _, inp := range v.Inputs() {
		var err error
		def, err = marshal(inp.Vertex(), def, cache, vertexCache, c)
		if err != nil {
			return def, err
		}
	}

	dgst, dt, opMeta, err := v.Marshal(c)
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
	def.Def = append(def.Def, dt)
	cache[dgst] = struct{}{}
	return def, nil
}

func (s State) Validate() error {
	return s.Output().Vertex().Validate()
}

func (s State) Output() Output {
	return s.out
}

func (s State) WithOutput(o Output) State {
	s = State{
		out: o,
		ctx: s.ctx,
	}
	s = s.ensurePlatform()
	return s
}

func (s State) Run(ro ...RunOption) ExecState {
	ei := &ExecInfo{State: s}
	if p := s.GetPlatform(); p != nil {
		ei.Constraints.Platform = p
	}
	for _, o := range ro {
		o.SetRunOption(ei)
	}
	meta := Meta{
		Args:     getArgs(ei.State),
		Cwd:      getDir(ei.State),
		Env:      getEnv(ei.State),
		User:     getUser(ei.State),
		ProxyEnv: ei.ProxyEnv,
	}

	exec := NewExecOp(s.Output(), meta, ei.ReadonlyRootFS, ei.Constraints)
	for _, m := range ei.Mounts {
		exec.AddMount(m.Target, m.Source, m.Opts...)
	}

	return ExecState{
		State: s.WithOutput(exec.Output()),
		exec:  exec,
	}
}

func (s State) AddEnv(key, value string) State {
	return s.AddEnvf(key, value)
}

func (s State) AddEnvf(key, value string, v ...interface{}) State {
	return addEnvf(key, value, v...)(s)
}

func (s State) Dir(str string) State {
	return s.Dirf(str)
}
func (s State) Dirf(str string, v ...interface{}) State {
	return dirf(str, v...)(s)
}

func (s State) GetEnv(key string) (string, bool) {
	return getEnv(s).Get(key)
}

func (s State) GetDir() string {
	return getDir(s)
}

func (s State) GetArgs() []string {
	return getArgs(s)
}

func (s State) Reset(s2 State) State {
	return reset(s2)(s)
}

func (s State) User(v string) State {
	return user(v)(s)
}

func (s State) Platform(p specs.Platform) State {
	return platform(p)(s)
}

func (s State) GetPlatform() *specs.Platform {
	return getPlatform(s)
}

func (s State) With(so ...StateOption) State {
	for _, o := range so {
		s = o(s)
	}
	return s
}

type output struct {
	vertex   Vertex
	getIndex func() (pb.OutputIndex, error)
	err      error
	platform *specs.Platform
}

func (o *output) ToInput(c *Constraints) (*pb.Input, error) {
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
	dgst, _, _, err := o.vertex.Marshal(c)
	if err != nil {
		return nil, err
	}
	return &pb.Input{Digest: dgst, Index: index}, nil
}

func (o *output) Vertex() Vertex {
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

	return m1
}

var IgnoreCache = constraintsOptFunc(func(c *Constraints) {
	c.Metadata.IgnoreCache = true
})

func WithDescription(m map[string]string) ConstraintsOpt {
	return constraintsOptFunc(func(c *Constraints) {
		c.Metadata.Description = m
	})
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
