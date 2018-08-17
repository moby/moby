/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package containerd

import (
	"context"
	"io"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type importOpts struct {
}

// ImportOpt allows the caller to specify import specific options
type ImportOpt func(c *importOpts) error

func resolveImportOpt(opts ...ImportOpt) (importOpts, error) {
	var iopts importOpts
	for _, o := range opts {
		if err := o(&iopts); err != nil {
			return iopts, err
		}
	}
	return iopts, nil
}

// Import imports an image from a Tar stream using reader.
// Caller needs to specify importer. Future version may use oci.v1 as the default.
// Note that unreferrenced blobs may be imported to the content store as well.
func (c *Client) Import(ctx context.Context, importer images.Importer, reader io.Reader, opts ...ImportOpt) ([]Image, error) {
	_, err := resolveImportOpt(opts...) // unused now
	if err != nil {
		return nil, err
	}

	ctx, done, err := c.WithLease(ctx)
	if err != nil {
		return nil, err
	}
	defer done(ctx)

	imgrecs, err := importer.Import(ctx, c.ContentStore(), reader)
	if err != nil {
		// is.Update() is not called on error
		return nil, err
	}

	is := c.ImageService()
	var images []Image
	for _, imgrec := range imgrecs {
		if updated, err := is.Update(ctx, imgrec, "target"); err != nil {
			if !errdefs.IsNotFound(err) {
				return nil, err
			}

			created, err := is.Create(ctx, imgrec)
			if err != nil {
				return nil, err
			}

			imgrec = created
		} else {
			imgrec = updated
		}

		images = append(images, NewImage(c, imgrec))
	}
	return images, nil
}

type exportOpts struct {
}

// ExportOpt allows the caller to specify export-specific options
type ExportOpt func(c *exportOpts) error

func resolveExportOpt(opts ...ExportOpt) (exportOpts, error) {
	var eopts exportOpts
	for _, o := range opts {
		if err := o(&eopts); err != nil {
			return eopts, err
		}
	}
	return eopts, nil
}

// Export exports an image to a Tar stream.
// OCI format is used by default.
// It is up to caller to put "org.opencontainers.image.ref.name" annotation to desc.
// TODO(AkihiroSuda): support exporting multiple descriptors at once to a single archive stream.
func (c *Client) Export(ctx context.Context, exporter images.Exporter, desc ocispec.Descriptor, opts ...ExportOpt) (io.ReadCloser, error) {
	_, err := resolveExportOpt(opts...) // unused now
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(exporter.Export(ctx, c.ContentStore(), desc, pw))
	}()
	return pr, nil
}
