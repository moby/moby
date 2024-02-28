package llbsolver

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/executor/resources"
	"github.com/moby/buildkit/exporter/containerimage"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver/ops"
	"github.com/moby/buildkit/solver/llbsolver/provenance"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type resultWithBridge struct {
	res    *frontend.Result
	bridge *provenanceBridge
}

// provenanceBridge provides scoped access to LLBBridge and captures the request it makes for provenance
type provenanceBridge struct {
	*llbBridge
	mu  sync.Mutex
	req *frontend.SolveRequest

	images     []provenancetypes.ImageSource
	builds     []resultWithBridge
	subBridges []*provenanceBridge
}

func (b *provenanceBridge) eachRef(f func(r solver.ResultProxy) error) error {
	for _, b := range b.builds {
		if err := b.res.EachRef(f); err != nil {
			return err
		}
	}
	for _, b := range b.subBridges {
		if err := b.eachRef(f); err != nil {
			return err
		}
	}
	return nil
}

func (b *provenanceBridge) allImages() []provenancetypes.ImageSource {
	res := make([]provenancetypes.ImageSource, 0, len(b.images))
	res = append(res, b.images...)
	for _, sb := range b.subBridges {
		res = append(res, sb.allImages()...)
	}
	return res
}

func (b *provenanceBridge) requests(r *frontend.Result) (*resultRequests, error) {
	reqs := &resultRequests{
		refs: make(map[string]*resultWithBridge),
		atts: make(map[string][]*resultWithBridge),
	}

	if r.Ref != nil {
		ref, ok := b.findByResult(r.Ref)
		if !ok {
			return nil, errors.Errorf("could not find request for ref %s", r.Ref.ID())
		}
		reqs.ref = ref
	}

	for k, ref := range r.Refs {
		if ref == nil {
			continue
		}
		r, ok := b.findByResult(ref)
		if !ok {
			return nil, errors.Errorf("could not find request for ref %s", ref.ID())
		}
		reqs.refs[k] = r
	}

	for k, atts := range r.Attestations {
		for _, att := range atts {
			if att.Ref == nil {
				continue
			}
			r, ok := b.findByResult(att.Ref)
			if !ok {
				return nil, errors.Errorf("could not find request for ref %s", att.Ref.ID())
			}
			reqs.atts[k] = append(reqs.atts[k], r)
		}
	}

	ps, err := exptypes.ParsePlatforms(r.Metadata)
	if err != nil {
		return nil, err
	}
	reqs.platforms = ps.Platforms

	return reqs, nil
}

func (b *provenanceBridge) findByResult(rp solver.ResultProxy) (*resultWithBridge, bool) {
	for _, br := range b.subBridges {
		if req, ok := br.findByResult(rp); ok {
			return req, true
		}
	}
	for _, bld := range b.builds {
		found := false
		bld.res.EachRef(func(r solver.ResultProxy) error {
			if r.ID() == rp.ID() {
				found = true
			}
			return nil
		})
		if found {
			return &bld, true
		}
	}
	return nil, false
}

func (b *provenanceBridge) ResolveSourceMetadata(ctx context.Context, op *pb.SourceOp, opt sourceresolver.Opt) (*sourceresolver.MetaResponse, error) {
	resp, err := b.llbBridge.ResolveSourceMetadata(ctx, op, opt)
	if err != nil {
		return nil, err
	}
	if img := resp.Image; img != nil {
		local := !strings.HasPrefix(resp.Op.Identifier, "docker-image://")
		ref := strings.TrimPrefix(resp.Op.Identifier, "docker-image://")
		ref = strings.TrimPrefix(ref, "oci-layout://")
		b.mu.Lock()
		b.images = append(b.images, provenancetypes.ImageSource{
			Ref:      ref,
			Platform: opt.Platform,
			Digest:   img.Digest,
			Local:    local,
		})
		b.mu.Unlock()
	}
	return resp, nil
}

func (b *provenanceBridge) Solve(ctx context.Context, req frontend.SolveRequest, sid string) (res *frontend.Result, err error) {
	if req.Definition != nil && req.Definition.Def != nil && req.Frontend != "" {
		return nil, errors.New("cannot solve with both Definition and Frontend specified")
	}

	if req.Definition != nil && req.Definition.Def != nil {
		rp := newResultProxy(b, req)
		res = &frontend.Result{Ref: rp}
		b.mu.Lock()
		b.builds = append(b.builds, resultWithBridge{res: res, bridge: b})
		b.mu.Unlock()
	} else if req.Frontend != "" {
		f, ok := b.llbBridge.frontends[req.Frontend]
		if !ok {
			return nil, errors.Errorf("invalid frontend: %s", req.Frontend)
		}
		wb := &provenanceBridge{llbBridge: b.llbBridge, req: &req}
		res, err = f.Solve(ctx, wb, b.llbBridge, req.FrontendOpt, req.FrontendInputs, sid, b.llbBridge.sm)
		if err != nil {
			return nil, err
		}
		wb.builds = append(wb.builds, resultWithBridge{res: res, bridge: wb})
		b.mu.Lock()
		b.subBridges = append(b.subBridges, wb)
		b.mu.Unlock()
	} else {
		return &frontend.Result{}, nil
	}
	if req.Evaluate {
		err = res.EachRef(func(ref solver.ResultProxy) error {
			_, err := ref.Result(ctx)
			return err
		})
	}
	return
}

type resultRequests struct {
	ref       *resultWithBridge
	refs      map[string]*resultWithBridge
	atts      map[string][]*resultWithBridge
	platforms []exptypes.Platform
}

// filterImagePlatforms filter out images that not for the current platform if an image exists for every platform in a result
func (reqs *resultRequests) filterImagePlatforms(k string, imgs []provenancetypes.ImageSource) []provenancetypes.ImageSource {
	if len(reqs.platforms) == 0 {
		return imgs
	}
	m := map[string]string{}
	for _, img := range imgs {
		if _, ok := m[img.Ref]; ok {
			continue
		}
		hasPlatform := true
		for _, p := range reqs.platforms {
			matcher := platforms.NewMatcher(p.Platform)
			found := false
			for _, img2 := range imgs {
				if img.Ref == img2.Ref && img2.Platform != nil {
					if matcher.Match(*img2.Platform) {
						found = true
						break
					}
				}
			}
			if !found {
				hasPlatform = false
				break
			}
		}
		if hasPlatform {
			m[img.Ref] = img.Ref
		}
	}

	var current ocispecs.Platform
	for _, p := range reqs.platforms {
		if p.ID == k {
			current = p.Platform
		}
	}

	out := make([]provenancetypes.ImageSource, 0, len(imgs))
	for _, img := range imgs {
		if _, ok := m[img.Ref]; ok && img.Platform != nil {
			if current.OS == img.Platform.OS && current.Architecture == img.Platform.Architecture {
				out = append(out, img)
			}
		} else {
			out = append(out, img)
		}
	}
	return out
}

func (reqs *resultRequests) allRes() map[string]struct{} {
	res := make(map[string]struct{})
	if reqs.ref != nil {
		res[reqs.ref.res.Ref.ID()] = struct{}{}
	}
	for _, r := range reqs.refs {
		res[r.res.Ref.ID()] = struct{}{}
	}
	for _, rs := range reqs.atts {
		for _, r := range rs {
			res[r.res.Ref.ID()] = struct{}{}
		}
	}
	return res
}

func captureProvenance(ctx context.Context, res solver.CachedResultWithProvenance) (*provenance.Capture, error) {
	if res == nil {
		return nil, nil
	}
	c := &provenance.Capture{}

	err := res.WalkProvenance(ctx, func(pp solver.ProvenanceProvider) error {
		switch op := pp.(type) {
		case *ops.SourceOp:
			id, pin := op.Pin()
			err := id.Capture(c, pin)
			if err != nil {
				return err
			}
		case *ops.ExecOp:
			pr := op.Proto()
			for _, m := range pr.Mounts {
				if m.MountType == pb.MountType_SECRET {
					c.AddSecret(provenancetypes.Secret{
						ID:       m.SecretOpt.GetID(),
						Optional: m.SecretOpt.GetOptional(),
					})
				}
				if m.MountType == pb.MountType_SSH {
					c.AddSSH(provenancetypes.SSH{
						ID:       m.SSHOpt.GetID(),
						Optional: m.SSHOpt.GetOptional(),
					})
				}
			}
			for _, se := range pr.Secretenv {
				c.AddSecret(provenancetypes.Secret{
					ID:       se.GetID(),
					Optional: se.GetOptional(),
				})
			}
			if pr.Network != pb.NetMode_NONE {
				c.NetworkAccess = true
			}
			samples, err := op.Samples()
			if err != nil {
				return err
			}
			if samples != nil {
				c.AddSamples(op.Digest(), samples)
			}
		case *ops.BuildOp:
			c.IncompleteMaterials = true // not supported yet
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return c, err
}

type ProvenanceCreator struct {
	pr        *provenancetypes.ProvenancePredicate
	j         *solver.Job
	sampler   *resources.SysSampler
	addLayers func() error
}

func NewProvenanceCreator(ctx context.Context, cp *provenance.Capture, res solver.ResultProxy, attrs map[string]string, j *solver.Job, usage *resources.SysSampler) (*ProvenanceCreator, error) {
	var reproducible bool
	if v, ok := attrs["reproducible"]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse reproducible flag %q", v)
		}
		reproducible = b
	}

	mode := "max"
	if v, ok := attrs["mode"]; ok {
		switch v {
		case "full":
			mode = "max"
		case "max", "min":
			mode = v
		default:
			return nil, errors.Errorf("invalid mode %q", v)
		}
	}

	withUsage := false
	if v, ok := attrs["capture-usage"]; ok {
		b, err := strconv.ParseBool(v)
		withUsage = err == nil && b
	}

	pr, err := provenance.NewPredicate(cp)
	if err != nil {
		return nil, err
	}

	st := j.StartedTime()

	pr.Metadata.BuildStartedOn = &st
	pr.Metadata.Reproducible = reproducible
	pr.Metadata.BuildInvocationID = j.UniqueID()

	pr.Builder.ID = attrs["builder-id"]

	var addLayers func() error

	switch mode {
	case "min":
		args := make(map[string]string)
		for k, v := range pr.Invocation.Parameters.Args {
			if strings.HasPrefix(k, "build-arg:") || strings.HasPrefix(k, "label:") {
				pr.Metadata.Completeness.Parameters = false
				continue
			}
			args[k] = v
		}
		pr.Invocation.Parameters.Args = args
		pr.Invocation.Parameters.Secrets = nil
		pr.Invocation.Parameters.SSH = nil
	case "max":
		dgsts, err := AddBuildConfig(ctx, pr, cp, res, withUsage)
		if err != nil {
			return nil, err
		}

		r, err := res.Result(ctx)
		if err != nil {
			return nil, err
		}

		wref, ok := r.Sys().(*worker.WorkerRef)
		if !ok {
			return nil, errors.Errorf("invalid worker ref %T", r.Sys())
		}

		addLayers = func() error {
			e := newCacheExporter()

			if wref.ImmutableRef != nil {
				ctx = withDescHandlerCacheOpts(ctx, wref.ImmutableRef)
			}

			if _, err := r.CacheKeys()[0].Exporter.ExportTo(ctx, e, solver.CacheExportOpt{
				ResolveRemotes: resolveRemotes,
				Mode:           solver.CacheExportModeRemoteOnly,
				ExportRoots:    true,
			}); err != nil {
				return err
			}

			m := map[string][][]ocispecs.Descriptor{}

			for l, descs := range e.layers {
				idx, ok := dgsts[l.digest]
				if !ok {
					continue
				}

				m[fmt.Sprintf("step%d:%d", idx, l.index)] = descs
			}

			if len(m) != 0 {
				if pr.Metadata == nil {
					pr.Metadata = &provenancetypes.ProvenanceMetadata{}
				}

				pr.Metadata.BuildKitMetadata.Layers = m
			}

			return nil
		}
	default:
		return nil, errors.Errorf("invalid mode %q", mode)
	}

	pc := &ProvenanceCreator{
		pr:        pr,
		j:         j,
		addLayers: addLayers,
	}
	if withUsage {
		pc.sampler = usage
	}
	return pc, nil
}

func (p *ProvenanceCreator) Predicate() (*provenancetypes.ProvenancePredicate, error) {
	end := p.j.RegisterCompleteTime()
	p.pr.Metadata.BuildFinishedOn = &end

	if p.addLayers != nil {
		if err := p.addLayers(); err != nil {
			return nil, err
		}
	}

	if p.sampler != nil {
		sysSamples, err := p.sampler.Close(true)
		if err != nil {
			return nil, err
		}
		p.pr.Metadata.BuildKitMetadata.SysUsage = sysSamples
	}

	return p.pr, nil
}

type edge struct {
	digest digest.Digest
	index  int
}

func newCacheExporter() *cacheExporter {
	return &cacheExporter{
		m:      map[interface{}]struct{}{},
		layers: map[edge][][]ocispecs.Descriptor{},
	}
}

type cacheExporter struct {
	layers map[edge][][]ocispecs.Descriptor
	m      map[interface{}]struct{}
}

func (ce *cacheExporter) Add(dgst digest.Digest) solver.CacheExporterRecord {
	return &cacheRecord{
		ce: ce,
	}
}

func (ce *cacheExporter) Visit(target any) {
	ce.m[target] = struct{}{}
}

func (ce *cacheExporter) Visited(target any) bool {
	_, ok := ce.m[target]
	return ok
}

type cacheRecord struct {
	ce *cacheExporter
}

func (c *cacheRecord) AddResult(dgst digest.Digest, idx int, createdAt time.Time, result *solver.Remote) {
	if result == nil || dgst == "" {
		return
	}
	e := edge{
		digest: dgst,
		index:  idx,
	}
	descs := make([]ocispecs.Descriptor, len(result.Descriptors))
	for i, desc := range result.Descriptors {
		d := desc
		d.Annotations = containerimage.RemoveInternalLayerAnnotations(d.Annotations, true)
		descs[i] = d
	}
	c.ce.layers[e] = appendLayerChain(c.ce.layers[e], descs)
}

func (c *cacheRecord) LinkFrom(rec solver.CacheExporterRecord, index int, selector string) {
}

func resolveRemotes(ctx context.Context, res solver.Result) ([]*solver.Remote, error) {
	ref, ok := res.Sys().(*worker.WorkerRef)
	if !ok {
		return nil, errors.Errorf("invalid result: %T", res.Sys())
	}

	remotes, err := ref.GetRemotes(ctx, false, config.RefConfig{}, true, nil)
	if err != nil {
		if errors.Is(err, cache.ErrNoBlobs) {
			return nil, nil
		}
		return nil, err
	}
	return remotes, nil
}

func AddBuildConfig(ctx context.Context, p *provenancetypes.ProvenancePredicate, c *provenance.Capture, rp solver.ResultProxy, withUsage bool) (map[digest.Digest]int, error) {
	def := rp.Definition()
	steps, indexes, err := toBuildSteps(def, c, withUsage)
	if err != nil {
		return nil, err
	}

	bc := &provenancetypes.BuildConfig{
		Definition:    steps,
		DigestMapping: digestMap(indexes),
	}

	p.BuildConfig = bc

	if def.Source != nil {
		sis := make([]provenancetypes.SourceInfo, len(def.Source.Infos))
		for i, si := range def.Source.Infos {
			steps, indexes, err := toBuildSteps(si.Definition, c, withUsage)
			if err != nil {
				return nil, err
			}
			s := provenancetypes.SourceInfo{
				Filename:      si.Filename,
				Data:          si.Data,
				Language:      si.Language,
				Definition:    steps,
				DigestMapping: digestMap(indexes),
			}
			sis[i] = s
		}

		if len(def.Source.Infos) != 0 {
			locs := map[string]*pb.Locations{}
			for k, l := range def.Source.Locations {
				idx, ok := indexes[digest.Digest(k)]
				if !ok {
					continue
				}
				locs[fmt.Sprintf("step%d", idx)] = l
			}

			if p.Metadata == nil {
				p.Metadata = &provenancetypes.ProvenanceMetadata{}
			}
			p.Metadata.BuildKitMetadata.Source = &provenancetypes.Source{
				Infos:     sis,
				Locations: locs,
			}
		}
	}

	return indexes, nil
}

func digestMap(idx map[digest.Digest]int) map[digest.Digest]string {
	m := map[digest.Digest]string{}
	for k, v := range idx {
		m[k] = fmt.Sprintf("step%d", v)
	}
	return m
}

func toBuildSteps(def *pb.Definition, c *provenance.Capture, withUsage bool) ([]provenancetypes.BuildStep, map[digest.Digest]int, error) {
	if def == nil || len(def.Def) == 0 {
		return nil, nil, nil
	}

	ops := make(map[digest.Digest]*pb.Op)
	defs := make(map[digest.Digest][]byte)

	var dgst digest.Digest
	for _, dt := range def.Def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return nil, nil, errors.Wrap(err, "failed to parse llb proto op")
		}
		if src := op.GetSource(); src != nil {
			for k := range src.Attrs {
				if k == "local.session" || k == "local.unique" {
					delete(src.Attrs, k)
				}
			}
		}
		dgst = digest.FromBytes(dt)
		ops[dgst] = &op
		defs[dgst] = dt
	}

	if dgst == "" {
		return nil, nil, nil
	}

	// depth first backwards
	dgsts := make([]digest.Digest, 0, len(def.Def))
	op := ops[dgst]

	if op.Op != nil {
		return nil, nil, errors.Errorf("invalid last vertex: %T", op.Op)
	}

	if len(op.Inputs) != 1 {
		return nil, nil, errors.Errorf("invalid last vertex inputs: %v", len(op.Inputs))
	}

	visited := map[digest.Digest]struct{}{}
	dgsts, err := walkDigests(dgsts, ops, dgst, visited)
	if err != nil {
		return nil, nil, err
	}
	indexes := map[digest.Digest]int{}
	for i, dgst := range dgsts {
		indexes[dgst] = i
	}

	out := make([]provenancetypes.BuildStep, 0, len(dgsts))
	for i, dgst := range dgsts {
		op := *ops[dgst]
		inputs := make([]string, len(op.Inputs))
		for i, inp := range op.Inputs {
			inputs[i] = fmt.Sprintf("step%d:%d", indexes[inp.Digest], inp.Index)
		}
		op.Inputs = nil
		s := provenancetypes.BuildStep{
			ID:     fmt.Sprintf("step%d", i),
			Inputs: inputs,
			Op:     op,
		}
		if withUsage {
			s.ResourceUsage = c.Samples[dgst]
		}
		out = append(out, s)
	}
	return out, indexes, nil
}

func walkDigests(dgsts []digest.Digest, ops map[digest.Digest]*pb.Op, dgst digest.Digest, visited map[digest.Digest]struct{}) ([]digest.Digest, error) {
	if _, ok := visited[dgst]; ok {
		return dgsts, nil
	}
	op, ok := ops[dgst]
	if !ok {
		return nil, errors.Errorf("failed to find input %v", dgst)
	}
	if op == nil {
		return nil, errors.Errorf("invalid nil input %v", dgst)
	}
	visited[dgst] = struct{}{}
	for _, inp := range op.Inputs {
		var err error
		dgsts, err = walkDigests(dgsts, ops, inp.Digest, visited)
		if err != nil {
			return nil, err
		}
	}
	dgsts = append(dgsts, dgst)
	return dgsts, nil
}

// appendLayerChain appends a layer chain to the set of layers while checking for duplicate layer chains.
func appendLayerChain(layers [][]ocispecs.Descriptor, descs []ocispecs.Descriptor) [][]ocispecs.Descriptor {
	for _, layerDescs := range layers {
		if len(layerDescs) != len(descs) {
			continue
		}

		matched := true
		for i, d := range layerDescs {
			if d.Digest != descs[i].Digest {
				matched = false
				break
			}
		}

		if matched {
			return layers
		}
	}
	return append(layers, descs)
}
