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

package oci

import (
	"context"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/snapshots"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Client interface used by SpecOpt
type Client interface {
	SnapshotService(snapshotterName string) snapshots.Snapshotter
}

// Image interface used by some SpecOpt to query image configuration
type Image interface {
	// Config descriptor for the image.
	Config(ctx context.Context) (ocispec.Descriptor, error)
	// ContentStore provides a content store which contains image blob data
	ContentStore() content.Store
}
