package cache

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

	cdcompression "github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/converter"
	"github.com/containerd/containerd/labels"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/compression"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// needsConversion indicates whether a conversion is needed for the specified descriptor to
// be the compressionType.
func needsConversion(ctx context.Context, cs content.Store, desc ocispecs.Descriptor, compressionType compression.Type) (bool, error) {
	mediaType := desc.MediaType
	switch compressionType {
	case compression.Uncompressed:
		if !images.IsLayerType(mediaType) || compression.FromMediaType(mediaType) == compression.Uncompressed {
			return false, nil
		}
	case compression.Gzip:
		esgz, err := isEStargz(ctx, cs, desc.Digest)
		if err != nil {
			return false, err
		}
		if (!images.IsLayerType(mediaType) || compression.FromMediaType(mediaType) == compression.Gzip) && !esgz {
			return false, nil
		}
	case compression.Zstd:
		if !images.IsLayerType(mediaType) || compression.FromMediaType(mediaType) == compression.Zstd {
			return false, nil
		}
	case compression.EStargz:
		esgz, err := isEStargz(ctx, cs, desc.Digest)
		if err != nil {
			return false, err
		}
		if !images.IsLayerType(mediaType) || esgz {
			return false, nil
		}
	default:
		return false, fmt.Errorf("unknown compression type during conversion: %q", compressionType)
	}
	return true, nil
}

// getConverter returns converter function according to the specified compression type.
// If no conversion is needed, this returns nil without error.
func getConverter(ctx context.Context, cs content.Store, desc ocispecs.Descriptor, comp compression.Config) (converter.ConvertFunc, error) {
	if needs, err := needsConversion(ctx, cs, desc, comp.Type); err != nil {
		return nil, errors.Wrapf(err, "failed to determine conversion needs")
	} else if !needs {
		// No conversion. No need to return an error here.
		return nil, nil
	}

	c := conversion{target: comp}

	from := compression.FromMediaType(desc.MediaType)
	switch from {
	case compression.Uncompressed:
	case compression.Gzip, compression.Zstd:
		c.decompress = func(ctx context.Context, desc ocispecs.Descriptor) (r io.ReadCloser, err error) {
			ra, err := cs.ReaderAt(ctx, desc)
			if err != nil {
				return nil, err
			}
			esgz, err := isEStargz(ctx, cs, desc.Digest)
			if err != nil {
				return nil, err
			} else if esgz {
				r, err = decompressEStargz(io.NewSectionReader(ra, 0, ra.Size()))
				if err != nil {
					return nil, err
				}
			} else {
				r, err = cdcompression.DecompressStream(io.NewSectionReader(ra, 0, ra.Size()))
				if err != nil {
					return nil, err
				}
			}
			return &readCloser{r, ra.Close}, nil
		}
	default:
		return nil, errors.Errorf("unsupported source compression type %q from mediatype %q", from, desc.MediaType)
	}

	switch comp.Type {
	case compression.Uncompressed:
	case compression.Gzip:
		c.compress = gzipWriter(comp)
	case compression.Zstd:
		c.compress = zstdWriter(comp)
	case compression.EStargz:
		compressorFunc, finalize := compressEStargz(comp)
		c.compress = func(w io.Writer) (io.WriteCloser, error) {
			return compressorFunc(w, ocispecs.MediaTypeImageLayerGzip)
		}
		c.finalize = finalize
	default:
		return nil, errors.Errorf("unknown target compression type during conversion: %q", comp.Type)
	}

	return (&c).convert, nil
}

type conversion struct {
	target     compression.Config
	decompress func(context.Context, ocispecs.Descriptor) (io.ReadCloser, error)
	compress   func(w io.Writer) (io.WriteCloser, error)
	finalize   func(context.Context, content.Store) (map[string]string, error)
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
	var zw io.WriteCloser = &nopWriteCloser{bufW}
	if c.compress != nil {
		zw, err = c.compress(zw)
		if err != nil {
			return nil, err
		}
	}
	zw = &onceWriteCloser{WriteCloser: zw}
	defer zw.Close()

	// convert this layer
	diffID := digest.Canonical.Digester()
	var rdr io.Reader
	if c.decompress == nil {
		ra, err := cs.ReaderAt(ctx, desc)
		if err != nil {
			return nil, err
		}
		defer ra.Close()
		rdr = io.NewSectionReader(ra, 0, ra.Size())
	} else {
		rc, err := c.decompress(ctx, desc)
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		rdr = rc
	}
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
	newDesc.MediaType = c.target.Type.DefaultMediaType()
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

type readCloser struct {
	io.ReadCloser
	closeFunc func() error
}

func (rc *readCloser) Close() error {
	err1 := rc.ReadCloser.Close()
	err2 := rc.closeFunc()
	if err1 != nil {
		return errors.Wrapf(err1, "failed to close: %v", err2)
	}
	return err2
}

type nopWriteCloser struct {
	io.Writer
}

func (w *nopWriteCloser) Close() error {
	return nil
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
