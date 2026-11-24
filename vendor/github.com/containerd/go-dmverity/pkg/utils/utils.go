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

package utils

import (
	"crypto"
	_ "crypto/sha1"   // register SHA1 for crypto.Hash
	_ "crypto/sha256" // register SHA256 for crypto.Hash
	_ "crypto/sha512" // register SHA512 for crypto.Hash
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
)

func GetBlockOrFileSize(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	mode := st.Mode()
	if (mode&os.ModeDevice) != 0 && (mode&os.ModeCharDevice) == 0 {
		f, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		var size uint64
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.BLKGETSIZE64, uintptr(unsafe.Pointer(&size)))
		if errno != 0 {
			return 0, errno
		}
		return int64(size), nil
	}
	return st.Size(), nil
}

func SelectHashSize(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "sha1":
		if crypto.SHA1.Available() {
			return crypto.SHA1.Size()
		}
	case "sha256":
		if crypto.SHA256.Available() {
			return crypto.SHA256.Size()
		}
	case "sha512":
		if crypto.SHA512.Available() {
			return crypto.SHA512.Size()
		}
	}
	return -1
}

func GetBitsDown(u uint32) uint {
	var i uint
	for (u >> i) > 1 {
		i++
	}
	return i
}

func AlignUp(x, align uint64) uint64 {
	if align == 0 {
		return x
	}
	rem := x % align
	if rem == 0 {
		return x
	}
	return x + (align - rem)
}

func ParseSaltHex(saltHex string, maxSaltSize int) ([]byte, error) {
	if saltHex == "" || saltHex == "-" {
		return nil, nil
	}

	b := make([]byte, hex.DecodedLen(len(saltHex)))
	n, err := hex.Decode(b, []byte(saltHex))
	if err != nil {
		return nil, fmt.Errorf("invalid salt hex: %w", err)
	}
	salt := b[:n]
	if len(salt) > maxSaltSize {
		return nil, fmt.Errorf("salt too large: %d > %d", len(salt), maxSaltSize)
	}
	return salt, nil
}

func ParseRootHash(rootHex string) ([]byte, error) {
	rootHex = strings.TrimSpace(rootHex)
	if rootHex == "" {
		return nil, fmt.Errorf("root hash is required")
	}

	rootBytes := make([]byte, hex.DecodedLen(len(rootHex)))
	n, err := hex.Decode(rootBytes, []byte(rootHex))
	if err != nil {
		return nil, fmt.Errorf("invalid root hex: %w", err)
	}
	return rootBytes[:n], nil
}

func ValidateRootHashSize(rootDigest []byte, hashName string) error {
	expectedHashSize := SelectHashSize(hashName)
	if expectedHashSize == 0 {
		expectedHashSize = crypto.SHA256.Size()
	}
	if len(rootDigest) != expectedHashSize {
		return fmt.Errorf("invalid root hash size: got %d bytes, expected %d bytes for %s",
			len(rootDigest), expectedHashSize, hashName)
	}
	return nil
}

func ValidateHashOffset(hashAreaOffset uint64, hashBlockSize uint32, noSuperblock bool) error {
	if noSuperblock && (hashAreaOffset%uint64(hashBlockSize) != 0) {
		return fmt.Errorf("hash offset %d must be aligned to hash block size %d",
			hashAreaOffset, hashBlockSize)
	}
	return nil
}

func ApplySalt(saltHex string, maxSaltSize int) ([]byte, uint16, error) {
	salt, err := ParseSaltHex(saltHex, maxSaltSize)
	if err != nil {
		return nil, 0, err
	}

	if salt != nil {
		return salt, uint16(len(salt)), nil
	}
	return nil, 0, nil
}

func ApplyUUID(uuidStr string, generateIfEmpty bool, noSuperblock bool, generateUUIDFunc func() (string, error)) ([16]byte, error) {
	var result [16]byte

	if uuidStr != "" {
		parsedUUID, err := uuid.Parse(uuidStr)
		if err != nil {
			return result, fmt.Errorf("invalid UUID format: %w", err)
		}
		copy(result[:], parsedUUID[:])
		return result, nil
	} else if generateIfEmpty && !noSuperblock {
		generatedUUID, err := generateUUIDFunc()
		if err != nil {
			return result, fmt.Errorf("failed to generate UUID: %w", err)
		}
		parsedUUID, err := uuid.Parse(generatedUUID)
		if err != nil {
			return result, fmt.Errorf("invalid generated UUID: %w", err)
		}
		copy(result[:], parsedUUID[:])
		return result, nil
	}

	return result, nil
}

func CalculateDataBlocks(dataPath string, userSpecified uint64, dataBlockSize uint32) (uint64, error) {
	if userSpecified != 0 {
		return userSpecified, nil
	}

	size, err := GetBlockOrFileSize(dataPath)
	if err != nil {
		return 0, fmt.Errorf("determine data device size: %w", err)
	}

	if size <= 0 {
		return 0, fmt.Errorf("cannot determine data size; provide --data-blocks")
	}

	if dataBlockSize == 0 {
		return 0, fmt.Errorf("data block size required")
	}

	if size%int64(dataBlockSize) != 0 {
		return 0, fmt.Errorf("data size %d not multiple of data block size %d",
			size, dataBlockSize)
	}

	return uint64(size / int64(dataBlockSize)), nil
}

func ValidateDataHashOverlap(dataBlocks uint64, dataBlockSize uint32, hashAreaOffset uint64, dataPath, hashPath string) error {
	if dataPath == hashPath && hashAreaOffset > 0 {
		dataAreaSize := dataBlocks * uint64(dataBlockSize)
		if dataAreaSize > hashAreaOffset {
			return fmt.Errorf("data area (size %d) overlaps with hash area (offset %d)",
				dataAreaSize, hashAreaOffset)
		}
	}
	return nil
}

func IsBlockSizeValid(size uint32) bool {
	return size%512 == 0 && size >= 512 && size <= (512*1024) && (size&(size-1)) == 0
}

func Uint64MultOverflow(a, b uint64) bool {
	if b == 0 {
		return false
	}
	result := a * b
	return result/b != a
}
