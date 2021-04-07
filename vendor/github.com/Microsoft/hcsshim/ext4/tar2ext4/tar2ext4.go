package tar2ext4

import (
	"archive/tar"
	"bufio"
	"encoding/binary"
	"io"
	"os"
	"path"
	"strings"

	"github.com/Microsoft/hcsshim/ext4/internal/compactext4"
	"github.com/Microsoft/hcsshim/ext4/internal/format"
)

type params struct {
	convertWhiteout bool
	appendVhdFooter bool
	ext4opts        []compactext4.Option
}

// Option is the type for optional parameters to Convert.
type Option func(*params)

// ConvertWhiteout instructs the converter to convert OCI-style whiteouts
// (beginning with .wh.) to overlay-style whiteouts.
func ConvertWhiteout(p *params) {
	p.convertWhiteout = true
}

// AppendVhdFooter instructs the converter to add a fixed VHD footer to the
// file.
func AppendVhdFooter(p *params) {
	p.appendVhdFooter = true
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

// Convert writes a compact ext4 file system image that contains the files in the
// input tar stream.
func Convert(r io.Reader, w io.ReadWriteSeeker, options ...Option) error {
	var p params
	for _, opt := range options {
		opt(&p)
	}
	t := tar.NewReader(bufio.NewReader(r))
	fs := compactext4.NewWriter(w, p.ext4opts...)
	for {
		hdr, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if p.convertWhiteout {
			dir, name := path.Split(hdr.Name)
			if strings.HasPrefix(name, whiteoutPrefix) {
				if name == opaqueWhiteout {
					// Update the directory with the appropriate xattr.
					f, err := fs.Stat(dir)
					if err != nil {
						return err
					}
					f.Xattrs["trusted.overlay.opaque"] = []byte("y")
					err = fs.Create(dir, f)
					if err != nil {
						return err
					}
				} else {
					// Create an overlay-style whiteout.
					f := &compactext4.File{
						Mode:     compactext4.S_IFCHR,
						Devmajor: 0,
						Devminor: 0,
					}
					err = fs.Create(path.Join(dir, name[len(whiteoutPrefix):]), f)
					if err != nil {
						return err
					}
				}

				continue
			}
		}

		if hdr.Typeflag == tar.TypeLink {
			err = fs.Link(hdr.Linkname, hdr.Name)
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
				Linkname: hdr.Linkname,
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
			case tar.TypeReg, tar.TypeRegA:
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
			err = fs.CreateWithParents(hdr.Name, f)
			if err != nil {
				return err
			}
			_, err = io.Copy(fs, t)
			if err != nil {
				return err
			}
		}
	}
	err := fs.Close()
	if err != nil {
		return err
	}
	if p.appendVhdFooter {
		size, err := w.Seek(0, io.SeekEnd)
		if err != nil {
			return err
		}
		err = binary.Write(w, binary.BigEndian, makeFixedVHDFooter(size))
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadExt4SuperBlock reads and returns ext4 super block from VHD
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
func ReadExt4SuperBlock(vhdPath string) (*format.SuperBlock, error) {
	vhd, err := os.OpenFile(vhdPath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer vhd.Close()

	// Skip padding at the start
	if _, err := vhd.Seek(1024, io.SeekStart); err != nil {
		return nil, err
	}
	var sb format.SuperBlock
	if err := binary.Read(vhd, binary.LittleEndian, &sb); err != nil {
		return nil, err
	}
	return &sb, nil
}
