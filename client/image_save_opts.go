package client

import (
	"fmt"
	"io"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ImageSaveOption interface {
	Apply(*imageSaveOpts) error
}

type imageSaveOptionFunc func(opt *imageSaveOpts) error

func (f imageSaveOptionFunc) Apply(o *imageSaveOpts) error {
	return f(o)
}

// ImageSaveWithPlatforms sets the platforms to be saved from the image.
func ImageSaveWithPlatforms(platforms ...ocispec.Platform) ImageSaveOption {
	return imageSaveOptionFunc(func(opt *imageSaveOpts) error {
		if opt.apiOptions.Platforms != nil {
			return fmt.Errorf("platforms already set to %v", opt.apiOptions.Platforms)
		}
		opt.apiOptions.Platforms = platforms
		return nil
	})
}

type imageSaveOpts struct {
	apiOptions imageSaveOptions
}

type imageSaveOptions struct {
	// Platforms selects the platforms to save if the image is a
	// multi-platform image and has multiple variants.
	Platforms []ocispec.Platform
}

func newImageSaveResult(rc io.ReadCloser) ImageSaveResult {
	if rc == nil {
		panic("nil io.ReadCloser")
	}
	return ImageSaveResult{
		rc:    rc,
		close: sync.OnceValue(rc.Close),
	}
}

type ImageSaveResult struct {
	rc    io.ReadCloser
	close func() error
}

// Read implements io.ReadCloser
func (r ImageSaveResult) Read(p []byte) (n int, err error) {
	if r.rc == nil {
		return 0, io.EOF
	}
	return r.rc.Read(p)
}

// Close implements io.ReadCloser
func (r ImageSaveResult) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}
