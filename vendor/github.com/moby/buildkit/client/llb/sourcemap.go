package llb

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
)

type SourceMap struct {
	State      *State
	Definition *Definition
	Filename   string
	Data       []byte
}

func NewSourceMap(st *State, filename string, dt []byte) *SourceMap {
	return &SourceMap{
		State:    st,
		Filename: filename,
		Data:     dt,
	}
}

func (s *SourceMap) Location(r []*pb.Range) ConstraintsOpt {
	return constraintsOptFunc(func(c *Constraints) {
		if s == nil {
			return
		}
		c.SourceLocations = append(c.SourceLocations, &SourceLocation{
			SourceMap: s,
			Ranges:    r,
		})
	})
}

type SourceLocation struct {
	SourceMap *SourceMap
	Ranges    []*pb.Range
}

type sourceMapCollector struct {
	maps      []*SourceMap
	index     map[*SourceMap]int
	locations map[digest.Digest][]*SourceLocation
}

func newSourceMapCollector() *sourceMapCollector {
	return &sourceMapCollector{
		index:     map[*SourceMap]int{},
		locations: map[digest.Digest][]*SourceLocation{},
	}
}

func (smc *sourceMapCollector) Add(dgst digest.Digest, ls []*SourceLocation) {
	for _, l := range ls {
		idx, ok := smc.index[l.SourceMap]
		if !ok {
			idx = len(smc.maps)
			smc.maps = append(smc.maps, l.SourceMap)
		}
		smc.index[l.SourceMap] = idx
	}
	smc.locations[dgst] = ls
}

func (smc *sourceMapCollector) Marshal(ctx context.Context, co ...ConstraintsOpt) (*pb.Source, error) {
	s := &pb.Source{
		Locations: make(map[string]*pb.Locations),
	}
	for _, m := range smc.maps {
		def := m.Definition
		if def == nil && m.State != nil {
			var err error
			def, err = m.State.Marshal(ctx, co...)
			if err != nil {
				return nil, err
			}
			m.Definition = def
		}

		info := &pb.SourceInfo{
			Data:     m.Data,
			Filename: m.Filename,
		}

		if def != nil {
			info.Definition = def.ToPB()
		}

		s.Infos = append(s.Infos, info)
	}

	for dgst, locs := range smc.locations {
		pbLocs, ok := s.Locations[dgst.String()]
		if !ok {
			pbLocs = &pb.Locations{}
		}

		for _, loc := range locs {
			pbLocs.Locations = append(pbLocs.Locations, &pb.Location{
				SourceIndex: int32(smc.index[loc.SourceMap]),
				Ranges:      loc.Ranges,
			})
		}

		s.Locations[dgst.String()] = pbLocs
	}

	return s, nil
}
