// +build windows

package distribution

import (
	"encoding/json"
	"fmt"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/docker/image"
)

func detectBaseLayer(is image.Store, m *schema1.Manifest, rootFS *image.RootFS) error {
	v1img := &image.V1Image{}
	if err := json.Unmarshal([]byte(m.History[len(m.History)-1].V1Compatibility), v1img); err != nil {
		return err
	}
	if v1img.Parent == "" {
		return fmt.Errorf("Last layer %q does not have a base layer reference", v1img.ID)
	}
	// There must be an image that already references the baselayer.
	for _, img := range is.Map() {
		if img.RootFS.BaseLayerID() == v1img.Parent {
			rootFS.BaseLayer = img.RootFS.BaseLayer
			return nil
		}
	}
	return fmt.Errorf("Invalid base layer %q", v1img.Parent)
}
