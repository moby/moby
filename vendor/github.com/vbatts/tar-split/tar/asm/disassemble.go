package asm

import (
	"errors"
	"io"

	"github.com/vbatts/tar-split/archive/tar"
	"github.com/vbatts/tar-split/tar/storage"
)

// runInputTarStreamGoroutine is the goroutine entrypoint.
//
// It centralizes the goroutine protocol so the core parsing logic can be
// written as ordinary Go code that just "returns an error".
//
// Protocol guarantees:
//   - pW is always closed exactly once (CloseWithError(nil) == Close()).
//   - if done != nil, exactly one value is sent (nil on success, non-nil on failure).
//   - panics are converted into a non-nil error (and the panic is rethrown).
func runInputTarStreamGoroutine(outputRdr io.Reader, pW *io.PipeWriter, p storage.Packer, fp storage.FilePutter, done chan<- error) {
	// Default to a non-nil error so a panic can't accidentally look like success.
	err := errors.New("panic in runInputTarStream")
	defer func() {
		// CloseWithError(nil) is equivalent to Close().
		pW.CloseWithError(err)

		if done != nil {
			done <- err
		}

		// Preserve panic semantics while still ensuring the protocol above runs.
		if r := recover(); r != nil {
			panic(r)
		}
	}()

	err = runInputTarStream(outputRdr, p, fp)
}

// runInputTarStream drives tar-split parsing.
//
// It reads a tar stream from outputRdr and records tar-split metadata into the
// provided storage.Packer.
//
// Abort behavior: if the consumer closes the read end early, the tee reader will
// stop producing bytes (due to pipe write failure) and tar parsing will return
// an error. We propagate that error so the goroutine terminates promptly rather
// than draining the input stream for no benefit.
func runInputTarStream(outputRdr io.Reader, p storage.Packer, fp storage.FilePutter) error {
	tr := tar.NewReader(outputRdr)
	tr.RawAccounting = true

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			// Even when EOF is reached, there is often 1024 null bytes at the end
			// of an archive. Collect them too.
			if b := tr.RawBytes(); len(b) > 0 {
				if _, err := p.AddEntry(storage.Entry{
					Type:    storage.SegmentType,
					Payload: b,
				}); err != nil {
					return err
				}
			}
			break // Not return: we still need to drain any additional padding.
		}
		if hdr == nil {
			break // Not return: we still need to drain any additional padding.
		}

		if b := tr.RawBytes(); len(b) > 0 {
			if _, err := p.AddEntry(storage.Entry{
				Type:    storage.SegmentType,
				Payload: b,
			}); err != nil {
				return err
			}
		}

		var csum []byte
		if hdr.Size > 0 {
			_, csum, err = fp.Put(hdr.Name, tr)
			if err != nil {
				return err
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
		if _, err := p.AddEntry(entry); err != nil {
			return err
		}

		if b := tr.RawBytes(); len(b) > 0 {
			if _, err := p.AddEntry(storage.Entry{
				Type:    storage.SegmentType,
				Payload: b,
			}); err != nil {
				return err
			}
		}
	}

	// It is allowable, and not uncommon that there is further padding on
	// the end of an archive, apart from the expected 1024 null bytes. We
	// do this in chunks rather than in one go to avoid cases where a
	// maliciously crafted tar file tries to trick us into reading many GBs
	// into memory.
	const paddingChunkSize = 1024 * 1024
	var paddingChunk [paddingChunkSize]byte
	for {
		n, err := outputRdr.Read(paddingChunk[:])
		if n != 0 {
			if _, aerr := p.AddEntry(storage.Entry{
				Type:    storage.SegmentType,
				Payload: paddingChunk[:n],
			}); aerr != nil {
				return aerr
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	return nil
}

// newInputTarStreamCommon sets up the shared plumbing for NewInputTarStream and
// NewInputTarStreamWithDone.
//
// It constructs an io.Pipe and an io.TeeReader such that:
//
//   - The caller reads tar bytes from the returned *io.PipeReader.
//   - The background goroutine simultaneously reads the same stream from the
//     TeeReader to perform tar-split parsing and metadata packing.
//
// Abort and synchronization semantics:
//
//   - Closing the returned PipeReader causes the TeeReader to fail its write to
//     the pipe, which in turn causes the background goroutine to exit promptly.
//   - If withDone is true, a done channel is returned that receives exactly one
//     error value (nil on success) once the background goroutine has fully
//     terminated. This allows callers to safely wait until the input reader `r`
//     is no longer in use.
func newInputTarStreamCommon(
	r io.Reader,
	p storage.Packer,
	fp storage.FilePutter,
	done chan<- error,
) (pr *io.PipeReader) {
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
	pr, pw := io.Pipe()

	if fp == nil {
		fp = storage.NewDiscardFilePutter()
	}

	outputRdr := io.TeeReader(r, pw)
	go runInputTarStreamGoroutine(outputRdr, pw, p, fp, done)

	return pr
}

// NewInputTarStream wraps the Reader stream of a tar archive and provides a
// Reader stream of the same.
//
// In the middle it will pack the segments and file metadata to storage.Packer
// `p`.
//
// The storage.FilePutter is where payload of files in the stream are
// stashed. If this stashing is not needed, you can provide a nil
// storage.FilePutter. Since the checksumming is still needed, then a default
// of NewDiscardFilePutter will be used internally
//
// If callers need to be able to abort early and/or wait for goroutine termination,
// prefer NewInputTarStreamWithDone.
//
// Deprecated: This leaves a goroutine around if the consumer aborts without consuming
// the whole stream, and does not allow the caller to know when r is safe to deallocate
// or when p has written everything. Use NewInputTarStreamWithDone instead.
func NewInputTarStream(r io.Reader, p storage.Packer, fp storage.FilePutter) (io.Reader, error) {
	pr := newInputTarStreamCommon(r, p, fp, nil)
	return pr, nil
}

// NewInputTarStreamWithDone wraps the Reader stream of a tar archive and provides a
// Reader stream of the same.
//
// In the middle it will pack the segments and file metadata to storage.Packer `p`.
//
// It also returns a done channel that will receive exactly one error value
// (nil on success) when the internal goroutine has fully completed parsing
// the tar stream (including the final paddingChunk draining loop) and has
// finished writing all entries to `p`.
//
// The returned reader is an io.ReadCloser so callers can stop early; closing it
// aborts the pipe so the internal goroutine can terminate promptly (rather than
// hanging on a blocked pipe write).
//
// The caller is expected to consume the returned reader fully until EOF
// (not just the tar EOF marker); closing the returned reader earlier will
// cause the done channel to return a failure.
func NewInputTarStreamWithDone(r io.Reader, p storage.Packer, fp storage.FilePutter) (io.ReadCloser, <-chan error, error) {
	done := make(chan error, 1)
	pr := newInputTarStreamCommon(r, p, fp, done)
	return pr, done, nil
}
