package asm

import (
	"bytes"
	"fmt"
	"hash"
	"hash/crc64"
	"io"
	"sync"

	"github.com/vbatts/tar-split/tar/storage"
)

// NewOutputTarStream returns an io.ReadCloser that is an assembled tar archive
// stream.
//
// It takes a storage.FileGetter, for mapping the file payloads that are to be read in,
// and a storage.Unpacker, which has access to the rawbytes and file order
// metadata. With the combination of these two items, a precise assembled Tar
// archive is possible.
func NewOutputTarStream(fg storage.FileGetter, up storage.Unpacker) io.ReadCloser {
	// ... Since these are interfaces, this is possible, so let's not have a nil pointer
	if fg == nil || up == nil {
		return nil
	}
	pr, pw := io.Pipe()
	go func() {
		err := WriteOutputTarStream(fg, up, pw)
		if err != nil {
			pw.CloseWithError(err)
		} else {
			pw.Close()
		}
	}()
	return pr
}

// WriteOutputTarStream writes assembled tar archive to a writer.
func WriteOutputTarStream(fg storage.FileGetter, up storage.Unpacker, w io.Writer) error {
	// ... Since these are interfaces, this is possible, so let's not have a nil pointer
	if fg == nil || up == nil {
		return nil
	}
	var copyBuffer []byte
	var crcHash hash.Hash
	var crcSum []byte
	var multiWriter io.Writer
	for {
		entry, err := up.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch entry.Type {
		case storage.SegmentType:
			if _, err := w.Write(entry.Payload); err != nil {
				return err
			}
		case storage.FileType:
			if entry.Size == 0 {
				continue
			}
			fh, err := fg.Get(entry.GetName())
			if err != nil {
				return err
			}
			if crcHash == nil {
				crcHash = crc64.New(storage.CRCTable)
				crcSum = make([]byte, 8)
				multiWriter = io.MultiWriter(w, crcHash)
				copyBuffer = byteBufferPool.Get().([]byte)
				defer byteBufferPool.Put(copyBuffer)
			} else {
				crcHash.Reset()
			}

			if _, err := copyWithBuffer(multiWriter, fh, copyBuffer); err != nil {
				fh.Close()
				return err
			}

			if !bytes.Equal(crcHash.Sum(crcSum[:0]), entry.Payload) {
				// I would rather this be a comparable ErrInvalidChecksum or such,
				// but since it's coming through the PipeReader, the context of
				// _which_ file would be lost...
				fh.Close()
				return fmt.Errorf("file integrity checksum failed for %q", entry.GetName())
			}
			fh.Close()
		}
	}
}

var byteBufferPool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024)
	},
}

// copyWithBuffer is taken from stdlib io.Copy implementation
// https://github.com/golang/go/blob/go1.5.1/src/io/io.go#L367
func copyWithBuffer(dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}
