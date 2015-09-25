package asm

import (
	"bytes"
	"fmt"
	"hash/crc64"
	"io"

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
		for {
			entry, err := up.Next()
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			switch entry.Type {
			case storage.SegmentType:
				if _, err := pw.Write(entry.Payload); err != nil {
					pw.CloseWithError(err)
					return
				}
			case storage.FileType:
				if entry.Size == 0 {
					continue
				}
				fh, err := fg.Get(entry.GetName())
				if err != nil {
					pw.CloseWithError(err)
					return
				}
				c := crc64.New(storage.CRCTable)
				tRdr := io.TeeReader(fh, c)
				if _, err := io.Copy(pw, tRdr); err != nil {
					fh.Close()
					pw.CloseWithError(err)
					return
				}
				if !bytes.Equal(c.Sum(nil), entry.Payload) {
					// I would rather this be a comparable ErrInvalidChecksum or such,
					// but since it's coming through the PipeReader, the context of
					// _which_ file would be lost...
					fh.Close()
					pw.CloseWithError(fmt.Errorf("file integrity checksum failed for %q", entry.GetName()))
					return
				}
				fh.Close()
			}
		}
	}()
	return pr
}
