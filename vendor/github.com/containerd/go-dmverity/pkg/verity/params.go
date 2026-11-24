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

package verity

const (
	VeritySignature     = "verity\x00\x00"
	SuperblockSize      = 512
	VerityMaxHashType   = 1
	VerityMaxLevels     = 63
	VerityMaxDigestSize = 1024
	MaxSaltSize         = 256
	diskSectorSize      = 512
)

type Params struct {
	HashName       string
	DataBlockSize  uint32
	HashBlockSize  uint32
	DataBlocks     uint64
	HashType       uint32
	Salt           []byte
	SaltSize       uint16
	HashAreaOffset uint64
	NoSuperblock   bool
	UUID           [16]byte
}

func DefaultParams() Params {
	return Params{
		HashName: "sha256",
		HashType: 1,
	}
}

func IsBlockSizeValid(size uint32) bool {
	return size%512 == 0 && size >= 512 && size <= (512*1024) && (size&(size-1)) == 0
}
