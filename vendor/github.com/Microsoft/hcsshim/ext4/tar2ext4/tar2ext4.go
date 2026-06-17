package tar2ext4

import (
	"archive/tar"
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/Microsoft/hcsshim/ext4/dmverity"
	"github.com/Microsoft/hcsshim/ext4/internal/compactext4"
	"github.com/Microsoft/hcsshim/ext4/internal/format"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/pkg/errors"
)

type params struct {
	convertWhiteout     bool
	convertBackslash    bool
	appendVhdFooter     bool
	onlyAppendVhdFooter bool
	appendDMVerity      bool
	ext4opts            []compactext4.Option
}

// Option is the type for optional parameters to Convert.
type Option func(*params)

// ConvertWhiteout instructs the converter to convert OCI-style whiteouts
// (beginning with .wh.) to overlay-style whiteouts.
func ConvertWhiteout(p *params) {
	p.convertWhiteout = true
}

// ConvertBackslash instructs the converter to replace `\` in path names with `/`.
// This is useful if the tar file was created on Windows, where `\` is the filepath separator.
func ConvertBackslash(p *params) {
	p.convertBackslash = true
}

// AppendVhdFooter instructs the converter to add a fixed VHD footer to the
// file.
func AppendVhdFooter(p *params) {
	p.appendVhdFooter = true
}

// OnlyAppendVhdFooter instructs the converter not to convert but still to add a fixed VHD footer to the
// file.
func OnlyAppendVhdFooter(p *params) {
	p.onlyAppendVhdFooter = true
}

// AppendDMVerity instructs the converter to add a dmverity Merkle tree for
// the ext4 filesystem after the filesystem and before the optional VHD footer
func AppendDMVerity(p *params) {
	p.appendDMVerity = true
}

// InlineData instructs the converter to write small files into the inode
// structures directly. This creates smaller images but currently is not
// compatible with DAX.
func InlineData(p *params) {
	p.ext4opts = append(p.ext4opts, compactext4.InlineData)
}

// MaximumDiskSize instructs the writer to limit the disk size to the specified
// value. This also reserves enough metadata space for the specified disk size.
// If not provided, then 16GB is the default.
func MaximumDiskSize(size int64) Option {
	return func(p *params) {
		p.ext4opts = append(p.ext4opts, compactext4.MaximumDiskSize(size))
	}
}

const (
	whiteoutPrefix = ".wh."
	opaqueWhiteout = ".wh..wh..opq"
)

// ConvertTarToExt4 writes a compact ext4 file system image that contains the files in the
// input tar stream.
func ConvertTarToExt4(r io.Reader, w io.ReadWriteSeeker, options ...Option) error {
	var p params
	for _, opt := range options {
		opt(&p)
	}

	t := tar.NewReader(bufio.NewReader(r))
	fs := compactext4.NewWriter(w, p.ext4opts...)
	for {
		hdr, err := t.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		name := hdr.Name
		linkName := hdr.Linkname
		if p.convertBackslash {
			// compactext assumes all paths are `/` separated
			// unconditionally replace all instances of `/`, regardless of GOOS
			name = strings.ReplaceAll(name, `\`, "/")
			linkName = strings.ReplaceAll(linkName, `\`, "/")
		}

		if err = fs.MakeParents(name); err != nil {
			return errors.Wrapf(err, "failed to ensure parent directories for %s", name)
		}

		if p.convertWhiteout {
			dir, file := path.Split(name)
			if strings.HasPrefix(file, whiteoutPrefix) {
				if file == opaqueWhiteout {
					// Update the directory with the appropriate xattr.
					f, err := fs.Stat(dir)
					if err != nil {
						return errors.Wrapf(err, "failed to stat parent directory of whiteout %s", file)
					}
					f.Xattrs["trusted.overlay.opaque"] = []byte("y")
					err = fs.Create(dir, f)
					if err != nil {
						return errors.Wrapf(err, "failed to create opaque dir %s", file)
					}
				} else {
					// Create an overlay-style whiteout.
					f := &compactext4.File{
						Mode:     compactext4.S_IFCHR,
						Devmajor: 0,
						Devminor: 0,
					}
					err = fs.Create(path.Join(dir, file[len(whiteoutPrefix):]), f)
					if err != nil {
						return errors.Wrapf(err, "failed to create whiteout file for %s", file)
					}
				}

				continue
			}
		}

		if hdr.Typeflag == tar.TypeLink {
			err = fs.Link(linkName, name)
			if err != nil {
				return err
			}
		} else {
			f := &compactext4.File{
				Mode:     uint16(hdr.Mode),
				Atime:    hdr.AccessTime,
				Mtime:    hdr.ModTime,
				Ctime:    hdr.ChangeTime,
				Crtime:   hdr.ModTime,
				Size:     hdr.Size,
				Uid:      uint32(hdr.Uid),
				Gid:      uint32(hdr.Gid),
				Linkname: linkName,
				Devmajor: uint32(hdr.Devmajor),
				Devminor: uint32(hdr.Devminor),
				Xattrs:   make(map[string][]byte),
			}
			for key, value := range hdr.PAXRecords {
				const xattrPrefix = "SCHILY.xattr."
				if strings.HasPrefix(key, xattrPrefix) {
					f.Xattrs[key[len(xattrPrefix):]] = []byte(value)
				}
			}

			var typ uint16
			switch hdr.Typeflag {
			case tar.TypeReg:
				typ = compactext4.S_IFREG
			case tar.TypeSymlink:
				typ = compactext4.S_IFLNK
			case tar.TypeChar:
				typ = compactext4.S_IFCHR
			case tar.TypeBlock:
				typ = compactext4.S_IFBLK
			case tar.TypeDir:
				typ = compactext4.S_IFDIR
			case tar.TypeFifo:
				typ = compactext4.S_IFIFO
			}
			f.Mode &= ^compactext4.TypeMask
			f.Mode |= typ
			err = fs.Create(name, f)
			if err != nil {
				return err
			}
			_, err = io.Copy(fs, t)
			if err != nil {
				return err
			}
		}
	}
	return fs.Close()
}

// Convert wraps ConvertTarToExt4 and conditionally computes (and appends) the file image's cryptographic
// hashes (merkle tree) or/and appends a VHD footer.
func Convert(r io.Reader, w io.ReadWriteSeeker, options ...Option) error {
	var p params
	for _, opt := range options {
		opt(&p)
	}

	if p.onlyAppendVhdFooter {
		_, err := io.Copy(w, r)
		if err != nil {
			return err
		}
		return ConvertToVhd(w)
	}

	if err := ConvertTarToExt4(r, w, options...); err != nil {
		return err
	}

	if p.appendDMVerity {
		if err := dmverity.ComputeAndWriteHashDevice(w, w); err != nil {
			return err
		}
	}

	if p.appendVhdFooter {
		return ConvertToVhd(w)
	}
	return nil
}

// ReadExt4SuperBlock reads and returns ext4 super block from given device.
func ReadExt4SuperBlock(devicePath string) (*format.SuperBlock, error) {
	dev, err := os.OpenFile(devicePath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer dev.Close()

	return ReadExt4SuperBlockReadSeeker(dev)
}

// ReadExt4SuperBlockReadSeeker reads and returns ext4 super block given
// an io.ReadSeeker.
//
// The layout on disk is as follows:
// | Group 0 padding     | - 1024 bytes
// | ext4 SuperBlock     | - 1 block
// | Group Descriptors   | - many blocks
// | Reserved GDT Blocks | - many blocks
// | Data Block Bitmap   | - 1 block
// | inode Bitmap        | - 1 block
// | inode Table         | - many blocks
// | Data Blocks         | - many blocks
//
// More details can be found here https://ext4.wiki.kernel.org/index.php/Ext4_Disk_Layout
//
// Our goal is to skip the Group 0 padding, read and return the ext4 SuperBlock
func ReadExt4SuperBlockReadSeeker(rsc io.ReadSeeker) (*format.SuperBlock, error) {
	// save current reader position
	currBytePos, err := rsc.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	if _, err := rsc.Seek(1024, io.SeekCurrent); err != nil {
		return nil, err
	}
	var sb format.SuperBlock
	if err := binary.Read(rsc, binary.LittleEndian, &sb); err != nil {
		return nil, err
	}

	// reset the reader to initial position
	if _, err := rsc.Seek(currBytePos, io.SeekStart); err != nil {
		return nil, err
	}

	if sb.Magic != format.SuperBlockMagic {
		return nil, errors.New("not an ext4 file system")
	}
	return &sb, nil
}

// IsDeviceExt4 is will read the device's superblock and determine if it is
// and ext4 superblock.
func IsDeviceExt4(devicePath string) bool {
	// ReadExt4SuperBlock will check the superblock magic number for us,
	// so we know if no error is returned, this is an ext4 device.
	_, err := ReadExt4SuperBlock(devicePath)
	if err != nil {
		log.L.Warnf("failed to read Ext4 superblock: %s", err)
	}
	return err == nil
}

// Ext4FileSystemSize reads ext4 superblock and returns the size of the underlying
// ext4 file system and its block size.
func Ext4FileSystemSize(r io.ReadSeeker) (int64, int, error) {
	sb, err := ReadExt4SuperBlockReadSeeker(r)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read ext4 superblock: %w", err)
	}
	blockSize := 1024 * (1 << sb.LogBlockSize)
	fsSize := int64(blockSize) * int64(sb.BlocksCountLow)
	return fsSize, blockSize, nil
}

// ConvertAndComputeRootDigest writes a compact ext4 file system image that contains the files in the
// input tar stream, computes the resulting file image's cryptographic hashes (merkle tree) and returns
// merkle tree root digest. Convert is called with minimal options: ConvertWhiteout and MaximumDiskSize
// set to dmverity.RecommendedVHDSizeGB.
func ConvertAndComputeRootDigest(r io.Reader) (string, error) {
	out, err := os.CreateTemp("", "")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		_ = os.Remove(out.Name())
	}()
	defer out.Close()

	options := []Option{
		ConvertWhiteout,
		MaximumDiskSize(dmverity.RecommendedVHDSizeGB),
	}
	if err := ConvertTarToExt4(r, out, options...); err != nil {
		return "", fmt.Errorf("failed to convert tar to ext4: %w", err)
	}

	if _, err := out.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to seek start on temp file when creating merkle tree: %w", err)
	}

	tree, err := dmverity.MerkleTree(bufio.NewReaderSize(out, dmverity.MerkleTreeBufioSize))
	if err != nil {
		return "", fmt.Errorf("failed to create merkle tree: %w", err)
	}

	hash := dmverity.RootHash(tree)
	return fmt.Sprintf("%x", hash), nil
}

// ConvertToVhd converts given io.WriteSeeker to VHD, by appending the VHD footer with a fixed size.
func ConvertToVhd(w io.WriteSeeker) error {
	size, err := w.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	return binary.Write(w, binary.BigEndian, makeFixedVHDFooter(size))
}

// A convenience wrapper for ConverToVhd, instead of asking the caller to open the file and pass an io.WriteSeeker, this
// takes in a file path and appends the VHD footer to that file.
func ConvertFileToVhd(filePath string) error {
	f, err := os.OpenFile(filePath, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file `%s` : %w", filePath, err)
	}
	defer f.Close()

	if err := ConvertToVhd(f); err != nil {
		return fmt.Errorf("failed to append VHD footer: %w", err)
	}
	return nil
}
