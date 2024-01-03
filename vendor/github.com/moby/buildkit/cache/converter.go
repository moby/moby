package cache

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images/converter"
	"github.com/containerd/containerd/labels"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/iohelper"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// getConverter returns converter function according to the specified compression type.
// If no conversion is needed, this returns nil without error.
func getConverter(ctx context.Context, cs content.Store, desc ocispecs.Descriptor, comp compression.Config) (converter.ConvertFunc, error) {
	if needs, err := comp.Type.NeedsConversion(ctx, cs, desc); err != nil {
		return nil, errors.Wrapf(err, "failed to determine conversion needs")
	} else if !needs {
		// No conversion. No need to return an error here.
		return nil, nil
	}

	from, err := compression.FromMediaType(desc.MediaType)
	if err != nil {
		return nil, err
	}

	c := conversion{target: comp}
	c.compress, c.finalize = comp.Type.Compress(ctx, comp)
	c.decompress = from.Decompress

	return (&c).convert, nil
}

type conversion struct {
	target     compression.Config
	decompress compression.Decompressor
	compress   compression.Compressor
	finalize   compression.Finalizer
}

var bufioPool = sync.Pool{
	New: func() interface{} {
		return nil
	},
}

func (c *conversion) convert(ctx context.Context, cs content.Store, desc ocispecs.Descriptor) (*ocispecs.Descriptor, error) {
	bklog.G(ctx).WithField("blob", desc).WithField("target", c.target).Debugf("converting blob to the target compression")
	// prepare the source and destination
	labelz := make(map[string]string)
	ref := fmt.Sprintf("convert-from-%s-to-%s-%s", desc.Digest, c.target.Type.String(), identity.NewID())
	w, err := cs.Writer(ctx, content.WithRef(ref))
	if err != nil {
		return nil, err
	}
	defer w.Close()
	if err := w.Truncate(0); err != nil { // Old written data possibly remains
		return nil, err
	}

	var bufW *bufio.Writer
	if pooledW := bufioPool.Get(); pooledW != nil {
		bufW = pooledW.(*bufio.Writer)
		bufW.Reset(w)
	} else {
		bufW = bufio.NewWriterSize(w, 128*1024)
	}
	defer bufioPool.Put(bufW)
	zw, err := c.compress(&iohelper.NopWriteCloser{Writer: bufW}, c.target.Type.MediaType())
	if err != nil {
		return nil, err
	}
	zw = &onceWriteCloser{WriteCloser: zw}
	defer zw.Close()

	// convert this layer
	diffID := digest.Canonical.Digester()
	rdr, err := c.decompress(ctx, cs, desc)
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	if _, err := io.Copy(zw, io.TeeReader(rdr, diffID.Hash())); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil { // Flush the writer
		return nil, err
	}
	if err := bufW.Flush(); err != nil { // Flush the buffer
		return nil, errors.Wrap(err, "failed to flush diff during conversion")
	}
	labelz[labels.LabelUncompressed] = diffID.Digest().String() // update diffID label
	if err = w.Commit(ctx, 0, "", content.WithLabels(labelz)); err != nil && !errdefs.IsAlreadyExists(err) {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	info, err := cs.Info(ctx, w.Digest())
	if err != nil {
		return nil, err
	}

	newDesc := desc
	newDesc.MediaType = c.target.Type.MediaType()
	newDesc.Digest = info.Digest
	newDesc.Size = info.Size
	newDesc.Annotations = map[string]string{labels.LabelUncompressed: diffID.Digest().String()}
	if c.finalize != nil {
		a, err := c.finalize(ctx, cs)
		if err != nil {
			return nil, errors.Wrapf(err, "failed finalize compression")
		}
		for k, v := range a {
			newDesc.Annotations[k] = v
		}
	}
	return &newDesc, nil
}

type onceWriteCloser struct {
	io.WriteCloser
	closeOnce sync.Once
}

func (w *onceWriteCloser) Close() (err error) {
	w.closeOnce.Do(func() {
		err = w.WriteCloser.Close()
	})
	return
}
