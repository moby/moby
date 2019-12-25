package contentutil

import (
	"context"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/remotes"
	"github.com/pkg/errors"
)

func FromPusher(p remotes.Pusher) content.Ingester {
	return &pushingIngester{
		p: p,
	}
}

type pushingIngester struct {
	p remotes.Pusher
}

// Writer implements content.Ingester. desc.MediaType must be set for manifest blobs.
func (i *pushingIngester) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	var wOpts content.WriterOpts
	for _, opt := range opts {
		if err := opt(&wOpts); err != nil {
			return nil, err
		}
	}
	if wOpts.Ref == "" {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "ref must not be empty")
	}
	// pusher requires desc.MediaType to determine the PUT URL, especially for manifest blobs.
	contentWriter, err := i.p.Push(ctx, wOpts.Desc)
	if err != nil {
		return nil, err
	}
	return &writer{
		Writer:           contentWriter,
		contentWriterRef: wOpts.Ref,
	}, nil
}

type writer struct {
	content.Writer          // returned from pusher.Push
	contentWriterRef string // ref passed for Writer()
}

func (w *writer) Status() (content.Status, error) {
	st, err := w.Writer.Status()
	if err != nil {
		return st, err
	}
	if w.contentWriterRef != "" {
		st.Ref = w.contentWriterRef
	}
	return st, nil
}
