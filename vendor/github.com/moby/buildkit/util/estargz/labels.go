package estargz

import (
	"fmt"
	"strings"

	ctdlabels "github.com/containerd/containerd/labels"
	"github.com/containerd/stargz-snapshotter/estargz"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func SnapshotLabels(ref string, descs []ocispecs.Descriptor, targetIndex int) map[string]string {
	if len(descs) < targetIndex {
		return nil
	}
	desc := descs[targetIndex]
	labels := make(map[string]string)
	for _, k := range []string{estargz.TOCJSONDigestAnnotation, estargz.StoreUncompressedSizeAnnotation} {
		labels[k] = desc.Annotations[k]
	}
	labels["containerd.io/snapshot/remote/stargz.reference"] = ref
	labels["containerd.io/snapshot/remote/stargz.digest"] = desc.Digest.String()
	var (
		layersKey = "containerd.io/snapshot/remote/stargz.layers"
		layers    string
	)
	for _, l := range descs[targetIndex:] {
		ls := fmt.Sprintf("%s,", l.Digest.String())
		// This avoids the label hits the size limitation.
		// Skipping layers is allowed here and only affects performance.
		if err := ctdlabels.Validate(layersKey, layers+ls); err != nil {
			break
		}
		layers += ls
	}
	labels[layersKey] = strings.TrimSuffix(layers, ",")
	return labels
}
