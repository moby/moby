package llbsolver

import (
	"slices"
	"sync"

	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver/provenance"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type provenanceStore struct {
	mu       sync.Mutex
	records  map[string]*provenanceRecord
	byDigest map[digest.Digest]map[string]struct{}
}

type provenanceRecord struct {
	defDigest digest.Digest
	request   *provenancetypes.RequestProvenance
}

func newProvenanceStore() *provenanceStore {
	return &provenanceStore{
		records:  map[string]*provenanceRecord{},
		byDigest: map[digest.Digest]map[string]struct{}{},
	}
}

func (s *provenanceStore) register(def *pb.Definition, req *provenancetypes.RequestProvenance) (string, digest.Digest, error) {
	if s == nil || def == nil || req == nil {
		return "", "", nil
	}
	dgst, err := definitionHeadDigest(def)
	if err != nil {
		return "", "", err
	}
	if dgst == "" {
		return "", "", nil
	}
	recordID := identity.NewID()
	s.mu.Lock()
	s.records[recordID] = &provenanceRecord{
		defDigest: dgst,
		request:   req.Clone(),
	}
	if s.byDigest[dgst] == nil {
		s.byDigest[dgst] = map[string]struct{}{}
	}
	s.byDigest[dgst][recordID] = struct{}{}
	s.mu.Unlock()
	return recordID, dgst, nil
}

func (s *provenanceStore) unregister(recordIDs []string) {
	if s == nil || len(recordIDs) == 0 {
		return
	}
	s.mu.Lock()
	for _, recordID := range recordIDs {
		rec := s.records[recordID]
		delete(s.records, recordID)
		if rec != nil {
			delete(s.byDigest[rec.defDigest], recordID)
			if len(s.byDigest[rec.defDigest]) == 0 {
				delete(s.byDigest, rec.defDigest)
			}
		}
	}
	s.mu.Unlock()
}

func (s *provenanceStore) lookup(def *pb.Definition) (*provenancetypes.RequestProvenance, bool) {
	if s == nil || def == nil {
		return nil, false
	}
	dgst, err := definitionHeadDigest(def)
	if err != nil || dgst == "" {
		return nil, false
	}
	s.mu.Lock()
	records := make([]*provenanceRecord, 0, len(s.byDigest[dgst]))
	for recordID := range s.byDigest[dgst] {
		if rec := s.records[recordID]; rec != nil && rec.request != nil {
			records = append(records, rec)
		}
	}
	s.mu.Unlock()
	if len(records) == 0 {
		return nil, false
	}
	req := records[0].request.Clone()
	if req.Request != nil {
		req.Request.Root = nil
	}
	for _, rec := range records[1:] {
		req2 := rec.request.Clone()
		if req2.Request != nil {
			req2.Request.Root = nil
		}
		if !req.Equal(req2) {
			return nil, false
		}
	}
	return req, true
}

func (b *provenanceBridge) registerProvenanceRefs(res *frontend.Result) error {
	if b == nil || res == nil {
		return nil
	}
	return res.EachRef(func(ref solver.ResultProxy) error {
		if ref == nil {
			return nil
		}
		return b.registerProvenanceRef(ref.Definition(), ref)
	})
}

func (b *provenanceBridge) registerProvenanceRef(def *pb.Definition, ref solver.ResultProxy) error {
	if def == nil || ref == nil || b.provenanceStore == nil {
		return nil
	}
	req, ok := b.findByResult(ref)
	if !ok {
		return nil
	}
	var srcs provenancetypes.Sources
	if p := ref.Provenance(); p != nil {
		c, ok := p.(*provenance.Capture)
		if !ok {
			return errors.Errorf("invalid provenance type %T", p)
		}
		if c != nil {
			srcs = c.Sources
		}
	}
	reqProv := req.bridge.requestProvenance(srcs)
	recordID, _, err := b.provenanceStore.register(def, reqProv)
	if err != nil {
		return err
	}
	if recordID == "" {
		return nil
	}
	b.addProvenanceRefRecordID(recordID)
	return nil
}

func (b *provenanceBridge) addProvenanceRefRecordID(recordID string) {
	if b == nil || recordID == "" {
		return
	}
	b.mu.Lock()
	b.provenanceRefRecordIDs = append(b.provenanceRefRecordIDs, recordID)
	b.mu.Unlock()
}

func (b *provenanceBridge) releaseProvenanceRefs() {
	if b == nil || b.provenanceStore == nil {
		return
	}
	_, subBridges, _ := b.snapshot()
	for _, sb := range subBridges {
		sb.releaseProvenanceRefs()
	}
	b.mu.Lock()
	recordIDs := slices.Clone(b.provenanceRefRecordIDs)
	b.provenanceRefRecordIDs = nil
	b.mu.Unlock()
	b.provenanceStore.unregister(recordIDs)
}

func (b *provenanceBridge) requestProvenance(srcs provenancetypes.Sources) *provenancetypes.RequestProvenance {
	if b == nil || b.req == nil {
		return nil
	}
	req := provenance.RequestProvenance(b.req.Frontend, provenance.FilterArgs(b.req.FrontendOpt), srcs)
	if req.Request == nil {
		req.Request = &provenancetypes.Parameters{}
	}
	if inputs := b.inputProvenance(b.req.FrontendInputs); len(inputs) > 0 {
		req.Request.Inputs = inputs
	}
	if root := b.rootRequestProvenance(srcs); root != nil && !root.Equal(req) {
		req.Request.Root = root
	}
	return req
}

func (b *provenanceBridge) inputProvenance(inputs map[string]*pb.Definition) map[string]*provenancetypes.RequestProvenance {
	if len(inputs) == 0 || b == nil || b.provenanceStore == nil {
		return nil
	}
	out := make(map[string]*provenancetypes.RequestProvenance)
	for name, def := range inputs {
		if in, ok := b.provenanceStore.lookup(def); ok {
			out[name] = in
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (b *provenanceBridge) rootRequestProvenance(srcs provenancetypes.Sources) *provenancetypes.RequestProvenance {
	if b == nil || !hasRequestProvenance(b.rootReq) {
		return nil
	}
	root := provenance.RequestProvenance(b.rootReq.Frontend, provenance.FilterArgs(b.rootReq.FrontendOpt), srcs)
	if root.Request == nil {
		root.Request = &provenancetypes.Parameters{}
	}
	if inputs := b.inputProvenance(b.rootReq.FrontendInputs); len(inputs) > 0 {
		root.Request.Inputs = inputs
	}
	if root.Request.Frontend == "" && len(root.Request.Args) == 0 && len(root.Request.Inputs) == 0 {
		return nil
	}
	return root
}

func hasRequestProvenance(req *frontend.SolveRequest) bool {
	if req == nil {
		return false
	}
	return req.Frontend != "" || len(req.FrontendOpt) > 0 || len(req.FrontendInputs) > 0
}

func definitionHeadDigest(def *pb.Definition) (digest.Digest, error) {
	if def == nil || len(def.Def) == 0 {
		return "", nil
	}
	var op pb.Op
	if err := op.UnmarshalVT(def.Def[len(def.Def)-1]); err != nil {
		return "", errors.Wrap(err, "failed to parse llb proto op")
	}
	if len(op.Inputs) == 0 {
		return "", nil
	}
	return digest.Digest(op.Inputs[0].Digest), nil
}
