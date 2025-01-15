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

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/pkg/ociwclayer"
	ocicimlayer "github.com/Microsoft/hcsshim/pkg/ociwclayer/cim"
)

// applyWindowsLayer applies a tar stream of an OCI style diff tar of a Windows layer
// See https://github.com/opencontainers/image-spec/blob/main/layer.md#applying-changesets
func applyWindowsLayer(ctx context.Context, root string, r io.Reader, options ApplyOptions) (size int64, err error) {
	// It seems that in certain situations, like having the containerd root and state on a file system hosted on a
	// mounted VHDX, we need SeSecurityPrivilege when opening a file with winio.ACCESS_SYSTEM_SECURITY. This happens
	// in the base layer writer in hcsshim when adding a new file.
	err = winio.RunWithPrivileges([]string{winio.SeSecurityPrivilege}, func() error {
		var innerErr error
		size, innerErr = ociwclayer.ImportLayerFromTar(ctx, r, root, options.Parents)
		return innerErr
	})
	return
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

func applyWindowsCimLayer(ctx context.Context, root string, r io.Reader, options ApplyOptions) (size int64, err error) {
	return ocicimlayer.ImportCimLayerFromTar(ctx, r, root, options.Parents)
}

// AsCimContainerLayer indicates that the tar stream to apply is that of a Windows container Layer written in
// the cim format.
func AsCimContainerLayer() ApplyOpt {
	return func(options *ApplyOptions) error {
		options.applyFunc = applyWindowsCimLayer
		return nil
	}
}
