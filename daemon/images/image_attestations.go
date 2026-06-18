package images

import (
	"context"
	"errors"

	imagetypes "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/v2/daemon/server/imagebackend"
	"github.com/moby/moby/v2/errdefs"
)

// ImageAttestations is not supported by the legacy image store.
func (i *ImageService) ImageAttestations(_ context.Context, _ string, _ imagebackend.AttestationOpts) ([]imagetypes.AttestationStatement, error) {
	return nil, errdefs.NotImplemented(errors.New("the legacy image store does not support attestations"))
}
