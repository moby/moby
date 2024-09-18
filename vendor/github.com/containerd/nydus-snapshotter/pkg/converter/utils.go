/*
 * Copyright (c) 2022. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package converter

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type File struct {
	Name   string
	Reader io.Reader
	Size   int64
}

type writeCloser struct {
	closed bool
	io.WriteCloser
	action func() error
}

func (c *writeCloser) Close() error {
	if c.closed {
		return nil
	}

	if err := c.WriteCloser.Close(); err != nil {
		return err
	}
	c.closed = true

	if err := c.action(); err != nil {
		return err
	}

	return nil
}

func newWriteCloser(wc io.WriteCloser, action func() error) *writeCloser {
	return &writeCloser{
		WriteCloser: wc,
		action:      action,
	}
}

type seekReader struct {
	io.ReaderAt
	pos int64
}

func (ra *seekReader) Read(p []byte) (int, error) {
	n, err := ra.ReaderAt.ReadAt(p, ra.pos)
	ra.pos += int64(n)
	return n, err
}

func (ra *seekReader) Seek(offset int64, whence int) (int64, error) {
	switch {
	case whence == io.SeekCurrent:
		ra.pos += offset
	case whence == io.SeekStart:
		ra.pos = offset
	default:
		return 0, fmt.Errorf("unsupported whence %d", whence)
	}

	return ra.pos, nil
}

func newSeekReader(ra io.ReaderAt) *seekReader {
	return &seekReader{
		ReaderAt: ra,
		pos:      0,
	}
}

// packToTar packs files to .tar(.gz) stream then return reader.
func packToTar(files []File, compress bool) io.ReadCloser {
	dirHdr := &tar.Header{
		Name:     "image",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}

	pr, pw := io.Pipe()

	go func() {
		// Prepare targz writer
		var tw *tar.Writer
		var gw *gzip.Writer
		var err error

		if compress {
			gw = gzip.NewWriter(pw)
			tw = tar.NewWriter(gw)
		} else {
			tw = tar.NewWriter(pw)
		}

		defer func() {
			err1 := tw.Close()
			var err2 error
			if gw != nil {
				err2 = gw.Close()
			}

			var finalErr error

			// Return the first error encountered to the other end and ignore others.
			switch {
			case err != nil:
				finalErr = err
			case err1 != nil:
				finalErr = err1
			case err2 != nil:
				finalErr = err2
			}

			pw.CloseWithError(finalErr)
		}()

		// Write targz stream
		if err = tw.WriteHeader(dirHdr); err != nil {
			return
		}

		for _, file := range files {
			hdr := tar.Header{
				Name: filepath.Join("image", file.Name),
				Mode: 0444,
				Size: file.Size,
			}
			if err = tw.WriteHeader(&hdr); err != nil {
				return
			}
			if _, err = io.Copy(tw, file.Reader); err != nil {
				return
			}
		}
	}()

	return pr
}

// Copied from containerd/containerd project, copyright The containerd Authors.
// https://github.com/containerd/containerd/blob/4902059cb554f4f06a8d06a12134c17117809f4e/images/converter/default.go#L385
func readJSON(ctx context.Context, cs content.Store, x interface{}, desc ocispec.Descriptor) (map[string]string, error) {
	info, err := cs.Info(ctx, desc.Digest)
	if err != nil {
		return nil, err
	}
	labels := info.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	b, err := content.ReadBlob(ctx, cs, desc)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, x); err != nil {
		return nil, err
	}
	return labels, nil
}

// Copied from containerd/containerd project, copyright The containerd Authors.
// https://github.com/containerd/containerd/blob/4902059cb554f4f06a8d06a12134c17117809f4e/images/converter/default.go#L401
func writeJSON(ctx context.Context, cs content.Store, x interface{}, oldDesc ocispec.Descriptor, labels map[string]string) (*ocispec.Descriptor, error) {
	b, err := json.Marshal(x)
	if err != nil {
		return nil, err
	}
	dgst := digest.SHA256.FromBytes(b)
	ref := fmt.Sprintf("converter-write-json-%s", dgst.String())
	w, err := content.OpenWriter(ctx, cs, content.WithRef(ref))
	if err != nil {
		return nil, err
	}
	if err := content.Copy(ctx, w, bytes.NewReader(b), int64(len(b)), dgst, content.WithLabels(labels)); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	newDesc := oldDesc
	newDesc.Size = int64(len(b))
	newDesc.Digest = dgst
	return &newDesc, nil
}
