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

package transfer

import (
	"context"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
)

type Transferrer interface {
	Transfer(ctx context.Context, source interface{}, destination interface{}, opts ...Opt) error
}

type ImageResolver interface {
	Resolve(ctx context.Context) (name string, desc ocispec.Descriptor, err error)
}

type ImageFetcher interface {
	ImageResolver

	Fetcher(ctx context.Context, ref string) (Fetcher, error)
}

type ImagePusher interface {
	Pusher(context.Context, ocispec.Descriptor) (Pusher, error)
}

type Fetcher interface {
	Fetch(context.Context, ocispec.Descriptor) (io.ReadCloser, error)
}

type Pusher interface {
	Push(context.Context, ocispec.Descriptor) (content.Writer, error)
}

// ImageFilterer is used to filter out child objects of an image
type ImageFilterer interface {
	ImageFilter(images.HandlerFunc, content.Store) images.HandlerFunc
}

// ImageStorer is a type which is capable of storing images for
// the provided descriptor. The descriptor may be any type of manifest
// including an index with multiple image references.
type ImageStorer interface {
	Store(context.Context, ocispec.Descriptor, images.Store) ([]images.Image, error)
}

// ImageGetter is type which returns an image from an image store
type ImageGetter interface {
	Get(context.Context, images.Store) (images.Image, error)
}

// ImageLookup is a type which returns images from an image store
// based on names or prefixes
type ImageLookup interface {
	Lookup(context.Context, images.Store) ([]images.Image, error)
}

// ImageExporter exports images to a writer
type ImageExporter interface {
	Export(context.Context, content.Store, []images.Image) error
}

// ImageImporter imports an image into a content store
type ImageImporter interface {
	Import(context.Context, content.Store) (ocispec.Descriptor, error)
}

// ImageImportStreamer returns an import streamer based on OCI or
// Docker image tar archives. The stream should be a raw tar stream
// and without compression.
type ImageImportStreamer interface {
	ImportStream(context.Context) (io.Reader, string, error)
}

type ImageExportStreamer interface {
	ExportStream(context.Context) (io.WriteCloser, string, error)
}

type ImageUnpacker interface {
	UnpackPlatforms() []UnpackConfiguration
}

// UnpackConfiguration specifies the platform and snapshotter to use for resolving
// the unpack Platform, if snapshotter is not specified the platform default will
// be used.
type UnpackConfiguration struct {
	Platform    ocispec.Platform
	Snapshotter string
}

type ProgressFunc func(Progress)

type Config struct {
	Progress ProgressFunc
}

type Opt func(*Config)

func WithProgress(f ProgressFunc) Opt {
	return func(opts *Config) {
		opts.Progress = f
	}
}

// Progress is used to represent a particular progress event or incremental
// update for the provided named object. The parents represent the names of
// the objects which initiated the progress for the provided named object.
// The name and what object it represents is determined by the implementation.
type Progress struct {
	Event    string
	Name     string
	Parents  []string
	Progress int64
	Total    int64
	// Descriptor?
}
