package tarexport

import (
	"testing"

	"github.com/docker/distribution"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/internal/layer"
	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
)

type fakeLayer struct {
	layer.Layer
	diffID layer.DiffID
}

func (l *fakeLayer) DiffID() layer.DiffID { return l.diffID }

// countingStore hands out one layer and counts how often it is taken and given back.
type countingStore struct {
	layer.Store
	l        layer.Layer
	gets     int
	releases int
}

func (s *countingStore) Get(layer.ChainID) (layer.Layer, error) {
	s.gets++
	return s.l, nil
}

func (s *countingStore) Release(layer.Layer) ([]layer.Metadata, error) {
	s.releases++
	return nil, nil
}

// An image may list the same diff ID more than once. The second time, the
// layer is already saved and saveConfigAndLayer returns early; the reference
// it took must still be released.
func TestSaveConfigAndLayerReleasesAlreadySavedLayer(t *testing.T) {
	diffID := digest.FromString("layer")
	lss := &countingStore{l: &fakeLayer{diffID: diffID}}
	s := &saveSession{
		tarexporter:  &tarexporter{lss: lss},
		outDir:       t.TempDir(),
		savedLayers:  map[layer.DiffID]distribution.Descriptor{diffID: {Digest: diffID}},
		savedConfigs: map[string]struct{}{"img": {}},
	}

	_, err := s.saveConfigAndLayer(t.Context(), diffID, image.V1Image{ID: "img"}, nil)
	assert.NilError(t, err)
	assert.Equal(t, lss.gets, 1)
	assert.Equal(t, lss.releases, 1)
}
