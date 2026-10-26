package snapshot

import (
	"testing"

	bksnapshot "github.com/moby/buildkit/snapshot"
	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
)

func TestChainID(t *testing.T) {
	chainID := digest.FromString("layer")
	s := &snapshotter{}

	for _, key := range []string{
		chainID.String(),
		bksnapshot.LayerSnapshotID(chainID),
	} {
		got, ok := s.chainID(key)
		assert.Assert(t, ok)
		assert.Equal(t, got, chainID)
	}

	for _, key := range []string{
		"not-a-snapshot",
		"buildkit-layer-snapshot-v1-sha256-invalid",
		"buildkit-layer-snapshot-v2-sha256-" + chainID.Encoded(),
	} {
		_, ok := s.chainID(key)
		assert.Assert(t, !ok)
	}
}

func TestLayerParentSnapshotID(t *testing.T) {
	chainID := digest.FromString("parent")
	assert.Equal(t, layerParentSnapshotID("sha256:"+digest.FromString("child").Encoded(), chainID), chainID.String())
	assert.Equal(t, layerParentSnapshotID(bksnapshot.LayerSnapshotID(digest.FromString("child")), chainID), bksnapshot.LayerSnapshotID(chainID))
}
