//go:build linux

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

package erofs

import (
	"context"

	"github.com/containerd/containerd/v2/internal/dmverity"
)

// getDmverityOptions returns dm-verity options configured for this differ instance.
// The block size is determined by the differ's mode:
// - Tar index mode requires 512-byte blocks to match EROFS and dm-verity constraints
// - Regular mode uses 4096-byte blocks (standard page size)
func (s *erofsDiff) getDmverityOptions() *dmverity.DmverityOptions {
	opts := dmverity.DefaultDmverityOptions()

	// Tar index mode requires 512-byte blocks because:
	// 1. EROFS tar index mode uses 512-byte metadata blocks (mkfs.erofs --tar=i)
	// 2. dm-verity sets the virtual block device logical_block_size to match the data block size
	// 3. EROFS requires its block size (512) to be >= the underlying block device's logical_block_size
	// Using 4096-byte dm-verity blocks would set logical_block_size=4096, causing EROFS sb_set_blocksize(512) to fail
	if s.enableTarIndex {
		opts.DataBlockSize = 512
		opts.HashBlockSize = 512
	}
	// Regular mode uses the default 4096-byte blocks (standard page size)

	return opts
}

// formatDmverityLayer formats an EROFS layer with a dm-verity hash tree using
// the differ's configured options.
func (s *erofsDiff) formatDmverityLayer(ctx context.Context, layerBlobPath string) error {
	return dmverity.FormatLayer(ctx, layerBlobPath, s.getDmverityOptions())
}
