package winlayers

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	keyFileAttr     = "MSWINDOWS.fileattr"
	keySDRaw        = "MSWINDOWS.rawsd"
	keyCreationTime = "LIBARCHIVE.creationtime"
)

func NewWalkingDiffWithWindows(store content.Store, d diff.Comparer) diff.Comparer {
	return &winDiffer{
		store: store,
		d:     d,
	}
}

var emptyDesc = ocispec.Descriptor{}

type winDiffer struct {
	store content.Store
	d     diff.Comparer
}

// Compare creates a diff between the given mounts and uploads the result
// to the content store.
func (s *winDiffer) Compare(ctx context.Context, lower, upper []mount.Mount, opts ...diff.Opt) (d ocispec.Descriptor, err error) {
	if !hasWindowsLayerMode(ctx) {
		return s.d.Compare(ctx, lower, upper, opts...)
	}

	var config diff.Config
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return emptyDesc, err
		}
	}

	if config.MediaType == "" {
		config.MediaType = ocispec.MediaTypeImageLayerGzip
	}

	var isCompressed bool
	switch config.MediaType {
	case ocispec.MediaTypeImageLayer:
	case ocispec.MediaTypeImageLayerGzip:
		isCompressed = true
	default:
		return emptyDesc, errors.Wrapf(errdefs.ErrNotImplemented, "unsupported diff media type: %v", config.MediaType)
	}

	var ocidesc ocispec.Descriptor
	if err := mount.WithTempMount(ctx, lower, func(lowerRoot string) error {
		return mount.WithTempMount(ctx, upper, func(upperRoot string) error {
			var newReference bool
			if config.Reference == "" {
				newReference = true
				config.Reference = uniqueRef()
			}

			cw, err := s.store.Writer(ctx,
				content.WithRef(config.Reference),
				content.WithDescriptor(ocispec.Descriptor{
					MediaType: config.MediaType, // most contentstore implementations just ignore this
				}))
			if err != nil {
				return errors.Wrap(err, "failed to open writer")
			}
			defer func() {
				if err != nil {
					cw.Close()
					if newReference {
						if err := s.store.Abort(ctx, config.Reference); err != nil {
							log.G(ctx).WithField("ref", config.Reference).Warnf("failed to delete diff upload")
						}
					}
				}
			}()
			if !newReference {
				if err := cw.Truncate(0); err != nil {
					return err
				}
			}

			if isCompressed {
				dgstr := digest.SHA256.Digester()
				compressed, err := compression.CompressStream(cw, compression.Gzip)
				if err != nil {
					return errors.Wrap(err, "failed to get compressed stream")
				}
				var w io.Writer = io.MultiWriter(compressed, dgstr.Hash())
				w, discard, done := makeWindowsLayer(w)
				err = archive.WriteDiff(ctx, w, lowerRoot, upperRoot)
				if err != nil {
					discard(err)
				}
				<-done
				compressed.Close()
				if err != nil {
					return errors.Wrap(err, "failed to write compressed diff")
				}

				if config.Labels == nil {
					config.Labels = map[string]string{}
				}
				config.Labels["containerd.io/uncompressed"] = dgstr.Digest().String()
			} else {
				w, discard, done := makeWindowsLayer(cw)
				if err = archive.WriteDiff(ctx, w, lowerRoot, upperRoot); err != nil {
					discard(err)
					return errors.Wrap(err, "failed to write diff")
				}
				<-done
			}

			var commitopts []content.Opt
			if config.Labels != nil {
				commitopts = append(commitopts, content.WithLabels(config.Labels))
			}

			dgst := cw.Digest()
			if err := cw.Commit(ctx, 0, dgst, commitopts...); err != nil {
				return errors.Wrap(err, "failed to commit")
			}

			info, err := s.store.Info(ctx, dgst)
			if err != nil {
				return errors.Wrap(err, "failed to get info from content store")
			}

			ocidesc = ocispec.Descriptor{
				MediaType: config.MediaType,
				Size:      info.Size,
				Digest:    info.Digest,
			}
			return nil
		})
	}); err != nil {
		return emptyDesc, err
	}

	return ocidesc, nil
}

func uniqueRef() string {
	t := time.Now()
	var b [3]byte
	// Ignore read failures, just decreases uniqueness
	rand.Read(b[:])
	return fmt.Sprintf("%d-%s", t.UnixNano(), base64.URLEncoding.EncodeToString(b[:]))
}

func prepareWinHeader(h *tar.Header) {
	if h.PAXRecords == nil {
		h.PAXRecords = map[string]string{}
	}
	if h.Typeflag == tar.TypeDir {
		h.Mode |= 1 << 14
		h.PAXRecords[keyFileAttr] = "16"
	}

	if h.Typeflag == tar.TypeReg {
		h.Mode |= 1 << 15
		h.PAXRecords[keyFileAttr] = "32"
	}

	if !h.ModTime.IsZero() {
		h.PAXRecords[keyCreationTime] = fmt.Sprintf("%d.%d", h.ModTime.Unix(), h.ModTime.Nanosecond())
	}

	h.Format = tar.FormatPAX
}

func addSecurityDescriptor(h *tar.Header) {
	if h.Typeflag == tar.TypeDir {
		// O:BAG:SYD:(A;OICI;FA;;;BA)(A;OICI;FA;;;SY)(A;;FA;;;BA)(A;OICIIO;GA;;;CO)(A;OICI;0x1200a9;;;BU)(A;CI;LC;;;BU)(A;CI;DC;;;BU)
		h.PAXRecords[keySDRaw] = "AQAEgBQAAAAkAAAAAAAAADAAAAABAgAAAAAABSAAAAAgAgAAAQEAAAAAAAUSAAAAAgCoAAcAAAAAAxgA/wEfAAECAAAAAAAFIAAAACACAAAAAxQA/wEfAAEBAAAAAAAFEgAAAAAAGAD/AR8AAQIAAAAAAAUgAAAAIAIAAAALFAAAAAAQAQEAAAAAAAMAAAAAAAMYAKkAEgABAgAAAAAABSAAAAAhAgAAAAIYAAQAAAABAgAAAAAABSAAAAAhAgAAAAIYAAIAAAABAgAAAAAABSAAAAAhAgAA"
	}

	if h.Typeflag == tar.TypeReg {
		// O:BAG:SYD:(A;;FA;;;BA)(A;;FA;;;SY)(A;;0x1200a9;;;BU)
		h.PAXRecords[keySDRaw] = "AQAEgBQAAAAkAAAAAAAAADAAAAABAgAAAAAABSAAAAAgAgAAAQEAAAAAAAUSAAAAAgBMAAMAAAAAABgA/wEfAAECAAAAAAAFIAAAACACAAAAABQA/wEfAAEBAAAAAAAFEgAAAAAAGACpABIAAQIAAAAAAAUgAAAAIQIAAA=="
	}
}

func makeWindowsLayer(w io.Writer) (io.Writer, func(error), chan error) {
	pr, pw := io.Pipe()
	done := make(chan error)

	go func() {
		tarReader := tar.NewReader(pr)
		tarWriter := tar.NewWriter(w)

		err := func() error {

			h := &tar.Header{
				Name:     "Hives",
				Typeflag: tar.TypeDir,
				ModTime:  time.Now(),
			}
			prepareWinHeader(h)
			if err := tarWriter.WriteHeader(h); err != nil {
				return err
			}

			h = &tar.Header{
				Name:     "Files",
				Typeflag: tar.TypeDir,
				ModTime:  time.Now(),
			}
			prepareWinHeader(h)
			if err := tarWriter.WriteHeader(h); err != nil {
				return err
			}

			for {
				h, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				h.Name = "Files/" + h.Name
				if h.Linkname != "" {
					h.Linkname = "Files/" + h.Linkname
				}
				prepareWinHeader(h)
				addSecurityDescriptor(h)
				if err := tarWriter.WriteHeader(h); err != nil {
					return err
				}
				if h.Size > 0 {
					if _, err := io.Copy(tarWriter, tarReader); err != nil {
						return err
					}
				}
			}
			return tarWriter.Close()
		}()
		if err != nil {
			logrus.Errorf("makeWindowsLayer %+v", err)
		}
		pw.CloseWithError(err)
		done <- err
		return
	}()

	discard := func(err error) {
		pw.CloseWithError(err)
	}

	return pw, discard, done
}
