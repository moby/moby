package asm

import (
	"bytes"
	"fmt"
	"io"

	"github.com/vbatts/tar-split/archive/tar"
	"github.com/vbatts/tar-split/tar/storage"
)

// IterateHeaders calls handler for each tar header provided by Unpacker
func IterateHeaders(unpacker storage.Unpacker, handler func(hdr *tar.Header) error) error {
	// We assume about NewInputTarStream:
	// - There is a separate SegmentType entry for every tar header, but only one SegmentType entry for the full header incl. any extensions
	// - (There is a FileType entry for every tar header, we ignore it)
	// - Trailing padding of a file, if any, is included in the next SegmentType entry
	// - At the end, there may be SegmentType entries just for the terminating zero blocks.

	var pendingPadding int64 = 0
	for {
		tsEntry, err := unpacker.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading tar-split entries: %w", err)
		}
		switch tsEntry.Type {
		case storage.SegmentType:
			payload := tsEntry.Payload
			if int64(len(payload)) < pendingPadding {
				return fmt.Errorf("expected %d bytes of padding after previous file, but next SegmentType only has %d bytes", pendingPadding, len(payload))
			}
			payload = payload[pendingPadding:]
			pendingPadding = 0

			tr := tar.NewReader(bytes.NewReader(payload))
			hdr, err := tr.Next()
			if err != nil {
				if err == io.EOF { // Probably the last entry, but let’s let the unpacker drive that.
					break
				}
				return fmt.Errorf("decoding a tar header from a tar-split entry: %w", err)
			}
			if err := handler(hdr); err != nil {
				return err
			}
			pendingPadding = tr.ExpectedPadding()

		case storage.FileType:
			// Nothing
		default:
			return fmt.Errorf("unexpected tar-split entry type %q", tsEntry.Type)
		}
	}
}
