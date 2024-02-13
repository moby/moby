package llb

import (
	"bytes"
	"context"

	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

// SourceMap maps a source file/location to an LLB state/definition.
// SourceMaps are used to provide information for debugging and helpful error messages to the user.
// As an example, lets say you have a Dockerfile with the following content:
//
//	FROM alpine
//	RUN exit 1
//
// When the "RUN" statement exits with a non-zero exit code buildkit will treat
// it as an error and is able to provide the user with a helpful error message
// pointing to exactly the line in the Dockerfile that caused the error.
type SourceMap struct {
	State      *State
	Definition *Definition
	Filename   string
	// Language should use names defined in https://github.com/github/linguist/blob/v7.24.1/lib/linguist/languages.yml
	Language string
	Data     []byte
}

func NewSourceMap(st *State, filename string, lang string, dt []byte) *SourceMap {
	return &SourceMap{
		State:    st,
		Filename: filename,
		Language: lang,
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

func equalSourceMap(sm1, sm2 *SourceMap) (out bool) {
	if sm1 == nil || sm2 == nil {
		return false
	}
	if sm1.Filename != sm2.Filename {
		return false
	}
	if sm1.Language != sm2.Language {
		return false
	}
	if len(sm1.Data) != len(sm2.Data) {
		return false
	}
	if !bytes.Equal(sm1.Data, sm2.Data) {
		return false
	}
	if sm1.Definition != nil && sm2.Definition != nil {
		if len(sm1.Definition.Def) != len(sm2.Definition.Def) && len(sm1.Definition.Def) != 0 {
			return false
		}
		if !bytes.Equal(sm1.Definition.Def[len(sm1.Definition.Def)-1], sm2.Definition.Def[len(sm2.Definition.Def)-1]) {
			return false
		}
	}
	return true
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
			idx = -1
			// slow equality check
			for i, m := range smc.maps {
				if equalSourceMap(m, l.SourceMap) {
					idx = i
					break
				}
			}
			if idx == -1 {
				idx = len(smc.maps)
				smc.maps = append(smc.maps, l.SourceMap)
			}
		}
		smc.index[l.SourceMap] = idx
	}
	smc.locations[dgst] = append(smc.locations[dgst], ls...)
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
			Language: m.Language,
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
