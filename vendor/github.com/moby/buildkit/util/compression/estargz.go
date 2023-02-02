package compression

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	cdcompression "github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/moby/buildkit/util/iohelper"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var EStargzAnnotations = []string{estargz.TOCJSONDigestAnnotation, estargz.StoreUncompressedSizeAnnotation}

const containerdUncompressed = "containerd.io/uncompressed"
const estargzLabel = "buildkit.io/compression/estargz"

func (c estargzType) Compress(ctx context.Context, comp Config) (compressorFunc Compressor, finalize Finalizer) {
	var cInfo *compressionInfo
	var writeErr error
	var mu sync.Mutex
	return func(dest io.Writer, requiredMediaType string) (io.WriteCloser, error) {
			ct, err := FromMediaType(requiredMediaType)
			if err != nil {
				return nil, err
			}
			if ct != Gzip {
				return nil, errors.Errorf("unsupported media type for estargz compressor %q", requiredMediaType)
			}
			done := make(chan struct{})
			pr, pw := io.Pipe()
			go func() (retErr error) {
				defer close(done)
				defer func() {
					if retErr != nil {
						mu.Lock()
						writeErr = retErr
						mu.Unlock()
					}
				}()

				blobInfoW, bInfoCh := calculateBlobInfo()
				defer blobInfoW.Close()
				level := gzip.DefaultCompression
				if comp.Level != nil {
					level = *comp.Level
				}
				w := estargz.NewWriterLevel(io.MultiWriter(dest, blobInfoW), level)

				// Using lossless API here to make sure that decompressEStargz provides the exact
				// same tar as the original.
				//
				// Note that we don't support eStragz compression for tar that contains a file named
				// `stargz.index.json` because we cannot create eStargz in loseless way for such blob
				// (we must overwrite stargz.index.json file).
				if err := w.AppendTarLossLess(pr); err != nil {
					pr.CloseWithError(err)
					return err
				}
				tocDgst, err := w.Close()
				if err != nil {
					pr.CloseWithError(err)
					return err
				}
				if err := blobInfoW.Close(); err != nil {
					pr.CloseWithError(err)
					return err
				}
				bInfo := <-bInfoCh
				mu.Lock()
				cInfo = &compressionInfo{bInfo, tocDgst}
				mu.Unlock()
				pr.Close()
				return nil
			}()
			return &iohelper.WriteCloser{WriteCloser: pw, CloseFunc: func() error {
				<-done // wait until the write completes
				return nil
			}}, nil
		}, func(ctx context.Context, cs content.Store) (map[string]string, error) {
			mu.Lock()
			cInfo, writeErr := cInfo, writeErr
			mu.Unlock()
			if cInfo == nil {
				if writeErr != nil {
					return nil, errors.Wrapf(writeErr, "cannot finalize due to write error")
				}
				return nil, errors.Errorf("cannot finalize (reason unknown)")
			}

			// Fill necessary labels
			info, err := cs.Info(ctx, cInfo.compressedDigest)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get info from content store")
			}
			if info.Labels == nil {
				info.Labels = make(map[string]string)
			}
			info.Labels[containerdUncompressed] = cInfo.uncompressedDigest.String()
			if _, err := cs.Update(ctx, info, "labels."+containerdUncompressed); err != nil {
				return nil, err
			}

			// Fill annotations
			a := make(map[string]string)
			a[estargz.TOCJSONDigestAnnotation] = cInfo.tocDigest.String()
			a[estargz.StoreUncompressedSizeAnnotation] = fmt.Sprintf("%d", cInfo.uncompressedSize)
			a[containerdUncompressed] = cInfo.uncompressedDigest.String()
			return a, nil
		}
}

func (c estargzType) Decompress(ctx context.Context, cs content.Store, desc ocispecs.Descriptor) (io.ReadCloser, error) {
	return decompress(ctx, cs, desc)
}

func (c estargzType) NeedsConversion(ctx context.Context, cs content.Store, desc ocispecs.Descriptor) (bool, error) {
	esgz, err := c.Is(ctx, cs, desc.Digest)
	if err != nil {
		return false, err
	}
	if !images.IsLayerType(desc.MediaType) || esgz {
		return false, nil
	}
	return true, nil
}

func (c estargzType) NeedsComputeDiffBySelf() bool {
	return true
}

func (c estargzType) OnlySupportOCITypes() bool {
	return true
}

func (c estargzType) NeedsForceCompression() bool {
	return false
}

func (c estargzType) MediaType() string {
	return ocispecs.MediaTypeImageLayerGzip
}

func (c estargzType) String() string {
	return "estargz"
}

// isEStargz returns true when the specified digest of content exists in
// the content store and it's eStargz.
func (c estargzType) Is(ctx context.Context, cs content.Store, dgst digest.Digest) (bool, error) {
	info, err := cs.Info(ctx, dgst)
	if err != nil {
		return false, nil
	}
	if isEsgzStr, ok := info.Labels[estargzLabel]; ok {
		if isEsgz, err := strconv.ParseBool(isEsgzStr); err == nil {
			return isEsgz, nil
		}
	}

	res := func() bool {
		r, err := cs.ReaderAt(ctx, ocispecs.Descriptor{Digest: dgst})
		if err != nil {
			return false
		}
		defer r.Close()
		sr := io.NewSectionReader(r, 0, r.Size())

		// Does this have the footer?
		tocOffset, _, err := estargz.OpenFooter(sr)
		if err != nil {
			return false
		}

		// Is TOC the final entry?
		decompressor := new(estargz.GzipDecompressor)
		rr, err := decompressor.Reader(io.NewSectionReader(sr, tocOffset, sr.Size()-tocOffset))
		if err != nil {
			return false
		}
		tr := tar.NewReader(rr)
		h, err := tr.Next()
		if err != nil {
			return false
		}
		if h.Name != estargz.TOCTarName {
			return false
		}
		if _, err = tr.Next(); err != io.EOF { // must be EOF
			return false
		}

		return true
	}()

	if info.Labels == nil {
		info.Labels = make(map[string]string)
	}
	info.Labels[estargzLabel] = strconv.FormatBool(res) // cache the result
	if _, err := cs.Update(ctx, info, "labels."+estargzLabel); err != nil {
		return false, err
	}

	return res, nil
}

func decompressEStargz(r *io.SectionReader) (io.ReadCloser, error) {
	return estargz.Unpack(r, new(estargz.GzipDecompressor))
}

type compressionInfo struct {
	blobInfo
	tocDigest digest.Digest
}

type blobInfo struct {
	compressedDigest   digest.Digest
	uncompressedDigest digest.Digest
	uncompressedSize   int64
}

func calculateBlobInfo() (io.WriteCloser, chan blobInfo) {
	res := make(chan blobInfo)
	pr, pw := io.Pipe()
	go func() {
		defer pr.Close()
		c := new(iohelper.Counter)
		dgstr := digest.Canonical.Digester()
		diffID := digest.Canonical.Digester()
		decompressR, err := cdcompression.DecompressStream(io.TeeReader(pr, dgstr.Hash()))
		if err != nil {
			pr.CloseWithError(err)
			return
		}
		defer decompressR.Close()
		if _, err := io.Copy(io.MultiWriter(c, diffID.Hash()), decompressR); err != nil {
			pr.CloseWithError(err)
			return
		}
		if err := decompressR.Close(); err != nil {
			pr.CloseWithError(err)
			return
		}
		res <- blobInfo{dgstr.Digest(), diffID.Digest(), c.Size()}
	}()
	return pw, res
}
