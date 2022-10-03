package containerd

import (
	"errors"
	"io"

	"github.com/docker/docker/errdefs"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImportImage imports an image, getting the archived layer data either from
// inConfig (if src is "-"), or from a URI specified in src. Progress output is
// written to outStream. Repository and tag names can optionally be given in
// the repo and tag arguments, respectively.
func (i *ImageService) ImportImage(src string, repository string, platform *specs.Platform, tag string, msg string, inConfig io.ReadCloser, outStream io.Writer, changes []string) error {
	return errdefs.NotImplemented(errors.New("not implemented"))
}
