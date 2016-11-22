package asm

import (
	"io"
	"io/ioutil"

	"github.com/vbatts/tar-split/archive/tar"
	"github.com/vbatts/tar-split/tar/storage"
)

// NewInputTarStream wraps the Reader stream of a tar archive and provides a
// Reader stream of the same.
//
// In the middle it will pack the segments and file metadata to storage.Packer
// `p`.
//
// The the storage.FilePutter is where payload of files in the stream are
// stashed. If this stashing is not needed, you can provide a nil
// storage.FilePutter. Since the checksumming is still needed, then a default
// of NewDiscardFilePutter will be used internally
func NewInputTarStream(r io.Reader, p storage.Packer, fp storage.FilePutter) (io.Reader, error) {
	// What to do here... folks will want their own access to the Reader that is
	// their tar archive stream, but we'll need that same stream to use our
	// forked 'archive/tar'.
	// Perhaps do an io.TeeReader that hands back an io.Reader for them to read
	// from, and we'll MITM the stream to store metadata.
	// We'll need a storage.FilePutter too ...

	// Another concern, whether to do any storage.FilePutter operations, such that we
	// don't extract any amount of the archive. But then again, we're not making
	// files/directories, hardlinks, etc. Just writing the io to the storage.FilePutter.
	// Perhaps we have a DiscardFilePutter that is a bit bucket.

	// we'll return the pipe reader, since TeeReader does not buffer and will
	// only read what the outputRdr Read's. Since Tar archives have padding on
	// the end, we want to be the one reading the padding, even if the user's
	// `archive/tar` doesn't care.
	pR, pW := io.Pipe()
	outputRdr := io.TeeReader(r, pW)

	// we need a putter that will generate the crc64 sums of file payloads
	if fp == nil {
		fp = storage.NewDiscardFilePutter()
	}

	go func() {
		tr := tar.NewReader(outputRdr)
		tr.RawAccounting = true
		for {
			hdr, err := tr.Next()
			if err != nil {
				if err != io.EOF {
					pW.CloseWithError(err)
					return
				}
				// even when an EOF is reached, there is often 1024 null bytes on
				// the end of an archive. Collect them too.
				if b := tr.RawBytes(); len(b) > 0 {
					_, err := p.AddEntry(storage.Entry{
						Type:    storage.SegmentType,
						Payload: b,
					})
					if err != nil {
						pW.CloseWithError(err)
						return
					}
				}
				break // not return. We need the end of the reader.
			}
			if hdr == nil {
				break // not return. We need the end of the reader.
			}

			if b := tr.RawBytes(); len(b) > 0 {
				_, err := p.AddEntry(storage.Entry{
					Type:    storage.SegmentType,
					Payload: b,
				})
				if err != nil {
					pW.CloseWithError(err)
					return
				}
			}

			var csum []byte
			if hdr.Size > 0 {
				var err error
				_, csum, err = fp.Put(hdr.Name, tr)
				if err != nil {
					pW.CloseWithError(err)
					return
				}
			}

			entry := storage.Entry{
				Type:    storage.FileType,
				Size:    hdr.Size,
				Payload: csum,
			}
			// For proper marshalling of non-utf8 characters
			entry.SetName(hdr.Name)

			// File entries added, regardless of size
			_, err = p.AddEntry(entry)
			if err != nil {
				pW.CloseWithError(err)
				return
			}

			if b := tr.RawBytes(); len(b) > 0 {
				_, err = p.AddEntry(storage.Entry{
					Type:    storage.SegmentType,
					Payload: b,
				})
				if err != nil {
					pW.CloseWithError(err)
					return
				}
			}
		}

		// it is allowable, and not uncommon that there is further padding on the
		// end of an archive, apart from the expected 1024 null bytes.
		remainder, err := ioutil.ReadAll(outputRdr)
		if err != nil && err != io.EOF {
			pW.CloseWithError(err)
			return
		}
		_, err = p.AddEntry(storage.Entry{
			Type:    storage.SegmentType,
			Payload: remainder,
		})
		if err != nil {
			pW.CloseWithError(err)
			return
		}
		pW.Close()
	}()

	return pR, nil
}
