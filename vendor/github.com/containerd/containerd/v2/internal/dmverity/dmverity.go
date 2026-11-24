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

// Package dmverity provides functions for working with dm-verity for integrity verification
// using the veritysetup-go library
package dmverity

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type DmverityOptions struct {
	// Salt for hashing, represented as a hex string
	Salt string
	// Hash algorithm to use (default: sha256)
	HashAlgorithm string
	// Size of data blocks in bytes (default: 4096)
	DataBlockSize uint32
	// Size of hash blocks in bytes (default: 4096)
	HashBlockSize uint32
	// Number of data blocks
	DataBlocks uint64
	// Offset of hash area in bytes
	HashOffset uint64
	// Hash type (default: 1)
	HashType uint32
	// NoSuperblock disables superblock usage (matches library's NoSuperblock field)
	NoSuperblock bool
	// UUID for device to use
	UUID string
}

func DefaultDmverityOptions() *DmverityOptions {
	return &DmverityOptions{
		Salt:          "0000000000000000000000000000000000000000000000000000000000000000",
		HashAlgorithm: "sha256",
		DataBlockSize: 4096,
		HashBlockSize: 4096,
		HashType:      1,
		NoSuperblock:  false, // By default, use superblock
	}
}

func MetadataPath(layerBlobPath string) string {
	if strings.HasSuffix(layerBlobPath, ".dmverity") {
		return layerBlobPath
	}
	return layerBlobPath + ".dmverity"
}

func DevicePath(name string) string {
	return fmt.Sprintf("/dev/mapper/%s", name)
}

type DmverityMetadata struct {
	RootHash   string `json:"roothash"`
	HashOffset uint64 `json:"hashoffset"`
}

func ReadMetadata(layerBlobPath string) (*DmverityMetadata, error) {
	metadataPath := MetadataPath(layerBlobPath)
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file %q: %w", metadataPath, err)
	}

	var metadata DmverityMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata file %q: %w", metadataPath, err)
	}

	if metadata.RootHash == "" {
		return nil, fmt.Errorf("missing root hash in metadata file %q", metadataPath)
	}

	return &metadata, nil
}
