package snapshot

import (
	"strings"

	digest "github.com/opencontainers/go-digest"
)

// Keep blob-backed layer snapshot IDs distinct from historical ChainID snapshot
// names. Older daemons could commit unverified layer contents under the
// advertised ChainID, so upgraded daemons must not collide with those objects.
const layerSnapshotIDPrefix = "buildkit-layer-snapshot-v1-"

// LayerSnapshotID returns the snapshot ID for a blob-backed layer identity.
// The chainID must be a valid digest.
func LayerSnapshotID(chainID digest.Digest) string {
	return layerSnapshotIDPrefix + chainID.Algorithm().String() + "-" + chainID.Encoded()
}

// ParseLayerSnapshotID parses a snapshot ID produced by LayerSnapshotID.
func ParseLayerSnapshotID(id string) (digest.Digest, bool) {
	encoded, ok := strings.CutPrefix(id, layerSnapshotIDPrefix)
	if !ok {
		return "", false
	}

	algorithm, encoded, ok := strings.Cut(encoded, "-")
	if !ok {
		return "", false
	}

	chainID, err := digest.Parse(algorithm + ":" + encoded)
	if err != nil {
		return "", false
	}
	return chainID, true
}
