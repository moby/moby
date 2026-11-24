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

package dmverity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/containerd/log"
	"github.com/google/uuid"

	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/pkg/atomicfile"
	"github.com/containerd/go-dmverity/pkg/utils"
	"github.com/containerd/go-dmverity/pkg/verity"
)

func IsSupported() (bool, error) {
	if _, err := os.Stat("/sys/module/dm_verity"); err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("dm_verity module not loaded or built-in")
		}
		return false, fmt.Errorf("failed to check /sys/module/dm_verity: %w", err)
	}

	return true, nil
}

func convertToVerityParams(opts *DmverityOptions) (verity.Params, error) {
	params := verity.DefaultParams()

	if opts != nil {
		if opts.HashAlgorithm != "" {
			params.HashName = opts.HashAlgorithm
		}
		if opts.DataBlockSize > 0 {
			params.DataBlockSize = opts.DataBlockSize
		}
		if opts.HashBlockSize > 0 {
			params.HashBlockSize = opts.HashBlockSize
		}
		if opts.DataBlocks > 0 {
			params.DataBlocks = opts.DataBlocks
		}
		if opts.HashOffset > 0 {
			params.HashAreaOffset = opts.HashOffset
		}
		if opts.HashType > 0 {
			params.HashType = opts.HashType
		}

		if opts.Salt != "" {
			salt, saltSize, err := utils.ApplySalt(opts.Salt, 256)
			if err != nil {
				return params, fmt.Errorf("invalid salt: %w", err)
			}
			params.Salt = salt
			params.SaltSize = saltSize
		}

		if opts.UUID != "" {
			uuidBytes, err := utils.ApplyUUID(opts.UUID, false, opts.NoSuperblock, nil)
			if err != nil {
				return params, fmt.Errorf("invalid UUID: %w", err)
			}
			params.UUID = uuidBytes
		}

		params.NoSuperblock = opts.NoSuperblock
	}

	return params, nil
}

// Format creates a dm-verity hash for a data device and returns the root hash.
// If hashDevice is the same as dataDevice, the hash will be stored on the same device.
func Format(dataDevice, hashDevice string, opts *DmverityOptions) (string, error) {
	if opts == nil {
		opts = DefaultDmverityOptions()
	}

	params, err := convertToVerityParams(opts)
	if err != nil {
		return "", fmt.Errorf("failed to convert options: %w", err)
	}

	if params.DataBlocks == 0 {
		size, err := utils.GetBlockOrFileSize(dataDevice)
		if err != nil {
			return "", fmt.Errorf("failed to get device size: %w", err)
		}
		params.DataBlocks = uint64(size / int64(params.DataBlockSize))
	}

	// IMPORTANT: This may modify params.HashAreaOffset when using superblock mode
	rootDigest, err := verity.Create(&params, dataDevice, hashDevice)
	if err != nil {
		return "", fmt.Errorf("failed to format dm-verity device: %w", err)
	}

	return fmt.Sprintf("%x", rootDigest), nil
}

// FormatLayer appends a dm-verity hash tree to the erofs layer blob at
// layerBlobPath (growing the file in place) and writes the "<blob>.dmverity"
// sidecar holding the resulting root hash and superblock offset. It is a no-op
// if the sidecar already exists. A nil opts uses DefaultDmverityOptions.
func FormatLayer(ctx context.Context, layerBlobPath string, opts *DmverityOptions) error {
	metadataPath := MetadataPath(layerBlobPath)
	if _, err := os.Stat(metadataPath); err == nil {
		log.G(ctx).WithField("path", layerBlobPath).Debug("Layer already formatted with dm-verity, skipping")
		return nil
	}

	if opts == nil {
		opts = DefaultDmverityOptions()
	} else {
		clone := *opts
		opts = &clone
	}

	fileInfo, err := os.Stat(layerBlobPath)
	if err != nil {
		return fmt.Errorf("failed to stat layer blob: %w", err)
	}

	blockSize := int64(opts.DataBlockSize)
	fileSize := fileInfo.Size()

	// dm-verity requires the hash area to start at a block-aligned offset
	dataBlocks := (fileSize + blockSize - 1) / blockSize
	hashOffset := uint64(dataBlocks * blockSize)

	opts.HashOffset = hashOffset
	opts.DataBlocks = uint64(dataBlocks)

	hashTreeSize, err := verity.GetHashTreeSize(&verity.Params{
		HashName:      opts.HashAlgorithm,
		DataBlockSize: opts.DataBlockSize,
		HashBlockSize: opts.HashBlockSize,
		DataBlocks:    opts.DataBlocks,
		HashType:      opts.HashType,
	})
	if err != nil {
		return fmt.Errorf("failed to calculate hash tree size: %w", err)
	}

	// In superblock mode, Format() stores the superblock at hashOffset and the hash tree after it
	superblockSize := uint64(0)
	if !opts.NoSuperblock {
		superblockSize = utils.AlignUp(uint64(verity.SuperblockSize), uint64(opts.HashBlockSize))
	}
	requiredSize := hashOffset + superblockSize + hashTreeSize
	if err := os.Truncate(layerBlobPath, int64(requiredSize)); err != nil {
		return fmt.Errorf("failed to pre-allocate space for hash tree: %w", err)
	}

	// Generate a random UUID for the superblock (required for superblock mode).
	// The library's ReadSuperblock() validates that UUID is not nil/empty.
	if opts.UUID == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("failed to generate superblock UUID: %w", err)
		}
		opts.UUID = u.String()
	}

	rootHash, err := Format(layerBlobPath, layerBlobPath, opts)
	if err != nil {
		return fmt.Errorf("failed to format dm-verity: %w", err)
	}

	// Save the ORIGINAL hashOffset (where the superblock is located), not the
	// post-Format offset (which points past the superblock). Open() needs the
	// superblock location to read device parameters.
	metadata := DmverityMetadata{
		RootHash:   rootHash,
		HashOffset: hashOffset,
	}
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal dm-verity metadata: %w", err)
	}
	// Write the sidecar atomically (temp file, synced, renamed) so a reader never
	// observes a partially written .dmverity.
	f, err := atomicfile.New(metadataPath, 0644)
	if err != nil {
		return fmt.Errorf("failed to create dm-verity metadata file: %w", err)
	}
	if _, err := f.Write(metadataBytes); err != nil {
		f.Cancel()
		return fmt.Errorf("failed to write dm-verity metadata: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to write dm-verity metadata: %w", err)
	}

	log.G(ctx).WithFields(log.Fields{
		"path":       layerBlobPath,
		"size":       fileSize,
		"blockSize":  opts.DataBlockSize,
		"hashOffset": hashOffset,
		"rootHash":   rootHash,
	}).Debug("Successfully formatted dm-verity layer")

	return nil
}

// Open creates a read-only device-mapper target for transparent integrity verification.
// It supports both superblock and no-superblock modes:
//
//   - Superblock mode (opts == nil or opts.NoSuperblock == false):
//     Reads dm-verity parameters from the superblock at the specified hashOffset.
//     Only rootHash needs to be provided; all other parameters are read from the device.
//     Use hashOffset to specify where the superblock is located (required when hash tree
//     is stored in the same file as data).
//
//   - No-superblock mode (opts != nil and opts.NoSuperblock == true):
//     Uses explicitly provided parameters from opts. All dm-verity parameters must be
//     supplied programmatically since there's no superblock to read from.
func Open(dataDevice string, name string, hashDevice string, rootHash string, hashOffset uint64, opts *DmverityOptions) (string, error) {
	if rootHash == "" {
		return "", fmt.Errorf("rootHash cannot be empty")
	}

	rootDigest, err := utils.ParseRootHash(rootHash)
	if err != nil {
		return "", fmt.Errorf("invalid root hash: %w", err)
	}

	var params verity.Params

	if opts != nil && opts.NoSuperblock {
		params, err = convertToVerityParams(opts)
		if err != nil {
			return "", fmt.Errorf("failed to convert options: %w", err)
		}
	} else {
		params = verity.DefaultParams()
		params.HashAreaOffset = hashOffset
	}

	loopParams := mount.LoopParams{
		Readonly:  true,
		Autoclear: true,
	}

	dataLoop, err := mount.SetupLoop(dataDevice, loopParams)
	if err != nil {
		return "", fmt.Errorf("failed to setup loop device for data: %w", err)
	}
	dataLoopDevice := dataLoop.Name()

	var hashLoop *os.File
	var hashLoopDevice string
	if hashDevice != dataDevice {
		hashLoop, err = mount.SetupLoop(hashDevice, loopParams)
		if err != nil {
			dataLoop.Close()
			return "", fmt.Errorf("failed to setup loop device for hash: %w", err)
		}
		hashLoopDevice = hashLoop.Name()
	} else {
		hashLoopDevice = dataLoopDevice
	}

	devicePath, err := verity.Open(&params, name, dataLoopDevice, hashLoopDevice, rootDigest, "", nil)
	if err != nil {
		dataLoop.Close()
		if hashLoop != nil {
			hashLoop.Close()
		}
		return "", fmt.Errorf("failed to open dm-verity device: %w", err)
	}

	// Close file handles now that dm-verity holds a kernel reference to the loop devices.
	dataLoop.Close()
	if hashLoop != nil {
		hashLoop.Close()
	}

	return devicePath, nil
}

func Close(name string) error {
	if err := verity.Close(name); err != nil {
		return fmt.Errorf("failed to close dm-verity device: %w", err)
	}
	return nil
}

// VerifyDevice ensures an existing dm-verity device matches the expected metadata and is healthy.
func VerifyDevice(name string, rootHash string) error {
	rootDigest, err := utils.ParseRootHash(rootHash)
	if err != nil {
		return fmt.Errorf("invalid root hash: %w", err)
	}

	// Use library's Check to verify device status and root hash
	if !verity.Check(name, rootDigest) {
		return fmt.Errorf("dm-verity device %q verification failed", name)
	}

	return nil
}
