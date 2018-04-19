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

package diff

import (
	"context"

	"github.com/containerd/containerd/mount"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Config is used to hold parameters needed for a diff operation
type Config struct {
	// MediaType is the type of diff to generate
	// Default depends on the differ,
	// i.e. application/vnd.oci.image.layer.v1.tar+gzip
	MediaType string

	// Reference is the content upload reference
	// Default will use a random reference string
	Reference string

	// Labels are the labels to apply to the generated content
	Labels map[string]string
}

// Opt is used to configure a diff operation
type Opt func(*Config) error

// Comparer allows creation of filesystem diffs between mounts
type Comparer interface {
	// Compare computes the difference between two mounts and returns a
	// descriptor for the computed diff. The options can provide
	// a ref which can be used to track the content creation of the diff.
	// The media type which is used to determine the format of the created
	// content can also be provided as an option.
	Compare(ctx context.Context, lower, upper []mount.Mount, opts ...Opt) (ocispec.Descriptor, error)
}

// Applier allows applying diffs between mounts
type Applier interface {
	// Apply applies the content referred to by the given descriptor to
	// the provided mount. The method of applying is based on the
	// implementation and content descriptor. For example, in the common
	// case the descriptor is a file system difference in tar format,
	// that tar would be applied on top of the mounts.
	Apply(ctx context.Context, desc ocispec.Descriptor, mount []mount.Mount) (ocispec.Descriptor, error)
}

// WithMediaType sets the media type to use for creating the diff, without
// specifying the differ will choose a default.
func WithMediaType(m string) Opt {
	return func(c *Config) error {
		c.MediaType = m
		return nil
	}
}

// WithReference is used to set the content upload reference used by
// the diff operation. This allows the caller to track the upload through
// the content store.
func WithReference(ref string) Opt {
	return func(c *Config) error {
		c.Reference = ref
		return nil
	}
}

// WithLabels is used to set content labels on the created diff content.
func WithLabels(labels map[string]string) Opt {
	return func(c *Config) error {
		c.Labels = labels
		return nil
	}
}
