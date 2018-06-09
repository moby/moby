package contentutil

import (
	"context"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/remotes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func FromFetcher(f remotes.Fetcher, desc ocispec.Descriptor) content.Provider {
	return &fetchedProvider{
		f:    f,
		desc: desc,
	}
}

type fetchedProvider struct {
	f    remotes.Fetcher
	desc ocispec.Descriptor
}

func (p *fetchedProvider) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	if desc.Digest != p.desc.Digest {
		return nil, errors.Wrapf(errdefs.ErrNotFound, "content %v", desc.Digest)
	}

	rc, err := p.f.Fetch(ctx, p.desc)
	if err != nil {
		return nil, err
	}

	return &readerAt{Reader: rc, Closer: rc, size: p.desc.Size}, nil
}

type readerAt struct {
	io.Reader
	io.Closer
	size   int64
	offset int64
}

func (r *readerAt) ReadAt(b []byte, off int64) (int, error) {
	if ra, ok := r.Reader.(io.ReaderAt); ok {
		return ra.ReadAt(b, off)
	}

	if r.offset != off {
		if seeker, ok := r.Reader.(io.Seeker); ok {
			if _, err := seeker.Seek(off, io.SeekStart); err != nil {
				return 0, err
			}
			r.offset = off
		} else {
			return 0, errors.Errorf("unsupported offset")
		}
	}

	var totalN int
	for len(b) > 0 {
		n, err := r.Reader.Read(b)
		r.offset += int64(n)
		totalN += n
		b = b[n:]
		if err != nil {
			return totalN, err
		}
	}
	return totalN, nil
}

func (r *readerAt) Size() int64 {
	return r.size
}
