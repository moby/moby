package dmverity

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"

	"github.com/Microsoft/hcsshim/ext4/internal/compactext4"
	"github.com/Microsoft/hcsshim/internal/memory"
)

const (
	blockSize = compactext4.BlockSize
	// MerkleTreeBufioSize is a default buffer size to use with bufio.Reader
	MerkleTreeBufioSize = memory.MiB // 1MB
	// RecommendedVHDSizeGB is the recommended size in GB for VHDs, which is not a hard limit.
	RecommendedVHDSizeGB = 128 * memory.GiB
	// VeritySignature is a value written to dm-verity super-block.
	VeritySignature = "verity"
)

var (
	salt   = bytes.Repeat([]byte{0}, 32)
	sbSize = binary.Size(dmveritySuperblock{})
)

var (
	ErrSuperBlockReadFailure  = errors.New("failed to read dm-verity super block")
	ErrSuperBlockParseFailure = errors.New("failed to parse dm-verity super block")
	ErrRootHashReadFailure    = errors.New("failed to read dm-verity root hash")
	ErrNotVeritySuperBlock    = errors.New("invalid dm-verity super-block signature")
)

type dmveritySuperblock struct {
	/* (0) "verity\0\0" */
	Signature [8]byte
	/* (8) superblock version, 1 */
	Version uint32
	/* (12) 0 - Chrome OS, 1 - normal */
	HashType uint32
	/* (16) UUID of hash device */
	UUID [16]byte
	/* (32) Name of the hash algorithm (e.g., sha256) */
	Algorithm [32]byte
	/* (64) The data block size in bytes */
	DataBlockSize uint32
	/* (68) The hash block size in bytes */
	HashBlockSize uint32
	/* (72) The number of data blocks */
	DataBlocks uint64
	/* (80) Size of the salt */
	SaltSize uint16
	/* (82) Padding */
	_ [6]byte
	/* (88) The salt */
	Salt [256]byte
	/* (344) Padding */
	_ [168]byte
}

// VerityInfo is minimal exported version of dmveritySuperblock
type VerityInfo struct {
	// Offset in blocks on hash device
	HashOffsetInBlocks int64
	// Set to true, when dm-verity super block is also written on the hash device
	SuperBlock    bool
	RootDigest    string
	Salt          string
	Algorithm     string
	DataBlockSize uint32
	HashBlockSize uint32
	DataBlocks    uint64
	Version       uint32
}

// MerkleTree constructs dm-verity hash-tree for a given io.Reader with a fixed salt (0-byte) and algorithm (sha256).
func MerkleTree(r io.Reader) ([]byte, error) {
	layers := make([][]byte, 0)
	currentLevel := r

	for {
		nextLevel := bytes.NewBuffer(make([]byte, 0))
		for {
			block := make([]byte, blockSize)
			if _, err := io.ReadFull(currentLevel, block); err != nil {
				if err == io.EOF {
					break
				}
				return nil, errors.Wrap(err, "failed to read data block")
			}
			h := hash2(salt, block)
			nextLevel.Write(h)
		}

		if nextLevel.Len()%blockSize != 0 {
			padding := bytes.Repeat([]byte{0}, blockSize-(nextLevel.Len()%blockSize))
			nextLevel.Write(padding)
		}

		layers = append(layers, nextLevel.Bytes())
		currentLevel = bufio.NewReaderSize(nextLevel, MerkleTreeBufioSize)

		// This means that only root hash remains and our job is done
		if nextLevel.Len() == blockSize {
			break
		}
	}

	tree := bytes.NewBuffer(make([]byte, 0))
	for i := len(layers) - 1; i >= 0; i-- {
		if _, err := tree.Write(layers[i]); err != nil {
			return nil, errors.Wrap(err, "failed to write merkle tree")
		}
	}

	return tree.Bytes(), nil
}

// RootHash computes root hash of dm-verity hash-tree
func RootHash(tree []byte) []byte {
	return hash2(salt, tree[:blockSize])
}

// NewDMVeritySuperblock returns a dm-verity superblock for a device with a given size, salt, algorithm and versions are
// fixed.
func NewDMVeritySuperblock(size uint64) *dmveritySuperblock {
	superblock := &dmveritySuperblock{
		Version:       1,
		HashType:      1,
		UUID:          generateUUID(),
		DataBlockSize: blockSize,
		HashBlockSize: blockSize,
		DataBlocks:    size / blockSize,
		SaltSize:      uint16(len(salt)),
	}

	copy(superblock.Signature[:], VeritySignature)
	copy(superblock.Algorithm[:], "sha256")
	copy(superblock.Salt[:], salt)

	return superblock
}

func hash2(a, b []byte) []byte {
	h := sha256.New()
	h.Write(append(a, b...))
	return h.Sum(nil)
}

func generateUUID() [16]byte {
	res := [16]byte{}
	if _, err := rand.Read(res[:]); err != nil {
		panic(err)
	}
	return res
}

// ReadDMVerityInfo extracts dm-verity super block information and merkle tree root hash
func ReadDMVerityInfo(vhdPath string, offsetInBytes int64) (*VerityInfo, error) {
	vhd, err := os.OpenFile(vhdPath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer vhd.Close()

	// Skip the ext4 data to get to dm-verity super block
	if s, err := vhd.Seek(offsetInBytes, io.SeekStart); err != nil || s != offsetInBytes {
		if err != nil {
			return nil, errors.Wrap(err, "failed to seek dm-verity super block")
		}
		return nil, errors.Errorf("failed to seek dm-verity super block: expected bytes=%d, actual=%d", offsetInBytes, s)
	}

	return ReadDMVerityInfoReader(vhd)
}

func ReadDMVerityInfoReader(r io.Reader) (*VerityInfo, error) {
	block := make([]byte, blockSize)
	if s, err := r.Read(block); err != nil || s != blockSize {
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSuperBlockReadFailure, err)
		}
		return nil, fmt.Errorf("unexpected bytes read expected=%d actual=%d: %w", blockSize, s, ErrSuperBlockReadFailure)
	}

	dmvSB := &dmveritySuperblock{}
	b := bytes.NewBuffer(block)
	if err := binary.Read(b, binary.LittleEndian, dmvSB); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSuperBlockParseFailure, err)
	}

	if string(bytes.Trim(dmvSB.Signature[:], "\x00")[:]) != VeritySignature {
		return nil, ErrNotVeritySuperBlock
	}

	if s, err := r.Read(block); err != nil || s != blockSize {
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrRootHashReadFailure, err)
		}
		return nil, fmt.Errorf("unexpected bytes read expected=%d, actual=%d: %w", blockSize, s, ErrRootHashReadFailure)
	}

	rootHash := hash2(dmvSB.Salt[:dmvSB.SaltSize], block)
	return &VerityInfo{
		RootDigest:         fmt.Sprintf("%x", rootHash),
		Algorithm:          string(bytes.Trim(dmvSB.Algorithm[:], "\x00")),
		Salt:               fmt.Sprintf("%x", dmvSB.Salt[:dmvSB.SaltSize]),
		HashOffsetInBlocks: int64(dmvSB.DataBlocks),
		SuperBlock:         true,
		DataBlocks:         dmvSB.DataBlocks,
		DataBlockSize:      dmvSB.DataBlockSize,
		HashBlockSize:      blockSize,
		Version:            dmvSB.Version,
	}, nil
}

// ComputeAndWriteHashDevice builds merkle tree from a given io.ReadSeeker and
// writes the result hash device (dm-verity super-block combined with merkle
// tree) to io.Writer.
func ComputeAndWriteHashDevice(r io.ReadSeeker, w io.Writer) error {
	// save current reader position
	currBytePos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	// reset to the beginning to find the device size
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return err
	}

	tree, err := MerkleTree(r)
	if err != nil {
		return errors.Wrap(err, "failed to build merkle tree")
	}

	devSize, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	// reset reader to initial position
	if _, err := r.Seek(currBytePos, io.SeekStart); err != nil {
		return err
	}

	dmVeritySB := NewDMVeritySuperblock(uint64(devSize))
	if err := binary.Write(w, binary.LittleEndian, dmVeritySB); err != nil {
		return errors.Wrap(err, "failed to write dm-verity super-block")
	}
	// write super-block padding
	padding := bytes.Repeat([]byte{0}, blockSize-(sbSize%blockSize))
	if _, err = w.Write(padding); err != nil {
		return err
	}
	// write tree
	if _, err := w.Write(tree); err != nil {
		return errors.Wrap(err, "failed to write merkle tree")
	}
	return nil
}
