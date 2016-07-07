// +build windows

package distribution

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/client/transport"
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
		if img.RootFS.Type == image.TypeLayersWithBase && img.RootFS.BaseLayerID() == v1img.Parent {
			rootFS.BaseLayer = img.RootFS.BaseLayer
			rootFS.Type = image.TypeLayersWithBase
			return nil
		}
	}
	return fmt.Errorf("Invalid base layer %q", v1img.Parent)
}

var _ distribution.Describable = &v2LayerDescriptor{}

func (ld *v2LayerDescriptor) Descriptor() distribution.Descriptor {
	if ld.src.MediaType == schema2.MediaTypeForeignLayer && len(ld.src.URLs) > 0 {
		return ld.src
	}
	return distribution.Descriptor{}
}

func (ld *v2LayerDescriptor) open(ctx context.Context) (distribution.ReadSeekCloser, error) {
	if len(ld.src.URLs) == 0 {
		blobs := ld.repo.Blobs(ctx)
		return blobs.Open(ctx, ld.digest)
	}

	var (
		err error
		rsc distribution.ReadSeekCloser
	)

	// Find the first URL that results in a 200 result code.
	for _, url := range ld.src.URLs {
		rsc = transport.NewHTTPReadSeeker(http.DefaultClient, url, nil)
		_, err = rsc.Seek(0, os.SEEK_SET)
		if err == nil {
			break
		}
		rsc.Close()
		rsc = nil
	}
	return rsc, err
}
