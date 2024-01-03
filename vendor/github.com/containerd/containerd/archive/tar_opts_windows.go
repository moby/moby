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

package archive

import (
	"context"
	"io"

	"github.com/Microsoft/hcsshim/pkg/ociwclayer"
)

// applyWindowsLayer applies a tar stream of an OCI style diff tar of a Windows layer
// See https://github.com/opencontainers/image-spec/blob/main/layer.md#applying-changesets
func applyWindowsLayer(ctx context.Context, root string, r io.Reader, options ApplyOptions) (size int64, err error) {
	return ociwclayer.ImportLayerFromTar(ctx, r, root, options.Parents)
}

// AsWindowsContainerLayer indicates that the tar stream to apply is that of
// a Windows Container Layer. The caller must be holding SeBackupPrivilege and
// SeRestorePrivilege.
func AsWindowsContainerLayer() ApplyOpt {
	return func(options *ApplyOptions) error {
		options.applyFunc = applyWindowsLayer
		return nil
	}
}

// writeDiffWindowsLayers writes a tar stream of the computed difference between the
// provided Windows layers
//
// Produces a tar using OCI style file markers for deletions. Deleted
// files will be prepended with the prefix ".wh.". This style is
// based off AUFS whiteouts.
// See https://github.com/opencontainers/image-spec/blob/main/layer.md
func writeDiffWindowsLayers(ctx context.Context, w io.Writer, _, layer string, options WriteDiffOptions) error {
	return ociwclayer.ExportLayerToTar(ctx, w, layer, options.ParentLayers)
}

// AsWindowsContainerLayerPair indicates that the paths to diff are a pair of
// Windows Container Layers. The caller must be holding SeBackupPrivilege.
func AsWindowsContainerLayerPair() WriteDiffOpt {
	return func(options *WriteDiffOptions) error {
		options.writeDiffFunc = writeDiffWindowsLayers
		return nil
	}
}

// WithParentLayers provides the Windows Container Layers that are the parents
// of the target (right-hand, "upper") layer, if any. The source (left-hand, "lower")
// layer passed to WriteDiff should be "" in this case.
func WithParentLayers(p []string) WriteDiffOpt {
	return func(options *WriteDiffOptions) error {
		options.ParentLayers = p
		return nil
	}
}
