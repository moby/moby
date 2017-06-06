package initrd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"

	"github.com/docker/linuxkit/src/pad4"
	"github.com/surma/gocpio"
)

// Writer is an io.WriteCloser that writes to an initrd
// This is a compressed cpio archive, zero padded to 4 bytes
type Writer struct {
	pw *pad4.Writer
	gw *gzip.Writer
	cw *cpio.Writer
}

func typeconv(thdr *tar.Header) int64 {
	switch thdr.Typeflag {
	case tar.TypeReg:
		return cpio.TYPE_REG
	case tar.TypeRegA:
		return cpio.TYPE_REG
	// Currently hard links not supported very well :)
	case tar.TypeLink:
		thdr.Linkname = "/" + thdr.Linkname
		return cpio.TYPE_SYMLINK
	case tar.TypeSymlink:
		return cpio.TYPE_SYMLINK
	case tar.TypeChar:
		return cpio.TYPE_CHAR
	case tar.TypeBlock:
		return cpio.TYPE_BLK
	case tar.TypeDir:
		return cpio.TYPE_DIR
	case tar.TypeFifo:
		return cpio.TYPE_FIFO
	default:
		return -1
	}
}

// CopyTar copies a tar stream into an initrd
func CopyTar(w *Writer, r *tar.Reader) (written int64, err error) {
	for {
		var thdr *tar.Header
		thdr, err = r.Next()
		if err == io.EOF {
			return written, nil
		}
		if err != nil {
			return
		}
		tp := typeconv(thdr)
		if tp == -1 {
			return written, errors.New("cannot convert tar file")
		}
		size := thdr.Size
		if tp == cpio.TYPE_SYMLINK {
			size = int64(len(thdr.Linkname))
		}
		chdr := cpio.Header{
			Mode:     thdr.Mode,
			Uid:      thdr.Uid,
			Gid:      thdr.Gid,
			Mtime:    thdr.ModTime.Unix(),
			Size:     size,
			Devmajor: thdr.Devmajor,
			Devminor: thdr.Devminor,
			Type:     tp,
			Name:     thdr.Name,
		}
		err = w.WriteHeader(&chdr)
		if err != nil {
			return
		}
		var n int64
		if tp == cpio.TYPE_SYMLINK {
			buffer := bytes.NewBufferString(thdr.Linkname)
			n, err = io.Copy(w, buffer)
		} else {
			n, err = io.Copy(w, r)
		}
		written += n
		if err != nil {
			return
		}
	}
}

// NewWriter creates a writer that will output an initrd stream
func NewWriter(w io.Writer) *Writer {
	initrd := new(Writer)
	initrd.pw = pad4.NewWriter(w)
	initrd.gw = gzip.NewWriter(initrd.pw)
	initrd.cw = cpio.NewWriter(initrd.gw)

	return initrd
}

// WriteHeader writes a cpio header into an initrd
func (w *Writer) WriteHeader(hdr *cpio.Header) error {
	return w.cw.WriteHeader(hdr)
}

// Write writes a cpio file into an initrd
func (w *Writer) Write(b []byte) (n int, e error) {
	return w.cw.Write(b)
}

// Close closes the writer
func (w *Writer) Close() error {
	err1 := w.cw.Close()
	err2 := w.gw.Close()
	err3 := w.pw.Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	if err3 != nil {
		return err3
	}
	return nil
}

// Copy reads a tarball in a stream and outputs a compressed init ram disk
func Copy(w *Writer, r io.Reader) (int64, error) {
	tr := tar.NewReader(r)

	return CopyTar(w, tr)
}
