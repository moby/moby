/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package walking

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/rand"
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
)

type walkingDiff struct {
	store content.Store
}

var emptyDesc = ocispec.Descriptor{}
var uncompressed = "containerd.io/uncompressed"

// NewWalkingDiff is a generic implementation of diff.Comparer.  The diff is
// calculated by mounting both the upper and lower mount sets and walking the
// mounted directories concurrently. Changes are calculated by comparing files
// against each other or by comparing file existence between directories.
// NewWalkingDiff uses no special characteristics of the mount sets and is
// expected to work with any filesystem.
func NewWalkingDiff(store content.Store) diff.Comparer {
	return &walkingDiff{
		store: store,
	}
}

// Compare creates a diff between the given mounts and uploads the result
// to the content store.
func (s *walkingDiff) Compare(ctx context.Context, lower, upper []mount.Mount, opts ...diff.Opt) (d ocispec.Descriptor, err error) {
	var config diff.Config
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return emptyDesc, err
		}
	}

	var isCompressed bool
	if config.Compressor != nil {
		if config.MediaType == "" {
			return emptyDesc, errors.New("media type must be explicitly specified when using custom compressor")
		}
		isCompressed = true
	} else {
		if config.MediaType == "" {
			config.MediaType = ocispec.MediaTypeImageLayerGzip
		}

		switch config.MediaType {
		case ocispec.MediaTypeImageLayer:
		case ocispec.MediaTypeImageLayerGzip:
			isCompressed = true
		default:
			return emptyDesc, fmt.Errorf("unsupported diff media type: %v: %w", config.MediaType, errdefs.ErrNotImplemented)
		}
	}

	var ocidesc ocispec.Descriptor
	if err := mount.WithTempMount(ctx, lower, func(lowerRoot string) error {
		return mount.WithReadonlyTempMount(ctx, upper, func(upperRoot string) error {
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
				return fmt.Errorf("failed to open writer: %w", err)
			}

			// errOpen is set when an error occurs while the content writer has not been
			// committed or closed yet to force a cleanup
			var errOpen error
			defer func() {
				if errOpen != nil {
					cw.Close()
					if newReference {
						if abortErr := s.store.Abort(ctx, config.Reference); abortErr != nil {
							log.G(ctx).WithError(abortErr).WithField("ref", config.Reference).Warnf("failed to delete diff upload")
						}
					}
				}
			}()
			if !newReference {
				if errOpen = cw.Truncate(0); errOpen != nil {
					return errOpen
				}
			}

			if isCompressed {
				dgstr := digest.SHA256.Digester()
				var compressed io.WriteCloser
				if config.Compressor != nil {
					compressed, errOpen = config.Compressor(cw, config.MediaType)
					if errOpen != nil {
						return fmt.Errorf("failed to get compressed stream: %w", errOpen)
					}
				} else {
					compressed, errOpen = compression.CompressStream(cw, compression.Gzip)
					if errOpen != nil {
						return fmt.Errorf("failed to get compressed stream: %w", errOpen)
					}
				}
				errOpen = archive.WriteDiff(ctx, io.MultiWriter(compressed, dgstr.Hash()), lowerRoot, upperRoot)
				compressed.Close()
				if errOpen != nil {
					return fmt.Errorf("failed to write compressed diff: %w", errOpen)
				}

				if config.Labels == nil {
					config.Labels = map[string]string{}
				}
				config.Labels[uncompressed] = dgstr.Digest().String()
			} else {
				if errOpen = archive.WriteDiff(ctx, cw, lowerRoot, upperRoot); errOpen != nil {
					return fmt.Errorf("failed to write diff: %w", errOpen)
				}
			}

			var commitopts []content.Opt
			if config.Labels != nil {
				commitopts = append(commitopts, content.WithLabels(config.Labels))
			}

			dgst := cw.Digest()
			if errOpen = cw.Commit(ctx, 0, dgst, commitopts...); errOpen != nil {
				if !errdefs.IsAlreadyExists(errOpen) {
					return fmt.Errorf("failed to commit: %w", errOpen)
				}
				errOpen = nil
			}

			info, err := s.store.Info(ctx, dgst)
			if err != nil {
				return fmt.Errorf("failed to get info from content store: %w", err)
			}
			if info.Labels == nil {
				info.Labels = make(map[string]string)
			}
			// Set uncompressed label if digest already existed without label
			if _, ok := info.Labels[uncompressed]; !ok {
				info.Labels[uncompressed] = config.Labels[uncompressed]
				if _, err := s.store.Update(ctx, info, "labels."+uncompressed); err != nil {
					return fmt.Errorf("error setting uncompressed label: %w", err)
				}
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
