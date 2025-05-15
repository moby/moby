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

package local

import (
	"context"
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/transfer"
	"github.com/containerd/containerd/v2/core/unpack"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
)

func (ts *localTransferService) importStream(ctx context.Context, i transfer.ImageImporter, is transfer.ImageStorer, tops *transfer.Config) error {
	ctx, done, err := ts.withLease(ctx)
	if err != nil {
		return err
	}
	defer done(ctx)

	if tops.Progress != nil {
		tops.Progress(transfer.Progress{
			Event: "Importing",
		})
	}

	index, err := i.Import(ctx, ts.content)
	if err != nil {
		return err
	}

	var (
		descriptors []ocispec.Descriptor
		handler     images.Handler
		unpacker    *unpack.Unpacker
	)

	// If save index, add index
	descriptors = append(descriptors, index)

	var handlerFunc images.HandlerFunc = func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		// Only save images at top level
		if desc.Digest != index.Digest {
			return images.Children(ctx, ts.content, desc)
		}

		p, err := content.ReadBlob(ctx, ts.content, desc)
		if err != nil {
			return nil, err
		}

		var idx ocispec.Index
		if err := json.Unmarshal(p, &idx); err != nil {
			return nil, err
		}

		for _, m := range idx.Manifests {
			m.Annotations = mergeMap(m.Annotations, map[string]string{"io.containerd.import.ref-source": "annotation"})
			descriptors = append(descriptors, m)
		}

		return idx.Manifests, nil
	}

	if f, ok := is.(transfer.ImageFilterer); ok {
		handlerFunc = f.ImageFilter(handlerFunc, ts.content)
	}

	handler = images.Handlers(handlerFunc)

	// First find suitable platforms to unpack into
	// If image storer is also an unpacker type, i.e implemented UnpackPlatforms() func
	if iu, ok := is.(transfer.ImageUnpacker); ok {
		unpacks := iu.UnpackPlatforms()
		if len(unpacks) > 0 {
			uopts := []unpack.UnpackerOpt{}
			for _, u := range unpacks {
				matched, mu := getSupportedPlatform(u, ts.config.UnpackPlatforms)
				if matched {
					uopts = append(uopts, unpack.WithUnpackPlatform(mu))
				}
			}

			if ts.config.DuplicationSuppressor != nil {
				uopts = append(uopts, unpack.WithDuplicationSuppressor(ts.config.DuplicationSuppressor))
			}
			unpacker, err = unpack.NewUnpacker(ctx, ts.content, uopts...)
			if err != nil {
				return fmt.Errorf("unable to initialize unpacker: %w", err)
			}
			handler = unpacker.Unpack(handler)
		}
	}

	if err := images.WalkNotEmpty(ctx, handler, index); err != nil {
		if unpacker != nil {
			// wait for unpacker to cleanup
			unpacker.Wait()
		}
		// TODO: Handle Not Empty as a special case on the input
		return err
	}

	if unpacker != nil {
		if _, err = unpacker.Wait(); err != nil {
			return err
		}
	}

	for _, desc := range descriptors {
		desc := desc
		imgs, err := is.Store(ctx, desc, ts.images)
		if err != nil {
			if errdefs.IsNotFound(err) {
				log.G(ctx).Infof("No images store for %s", desc.Digest)
				continue
			}
			return err
		}

		if tops.Progress != nil {
			for _, img := range imgs {
				tops.Progress(transfer.Progress{
					Event: "saved",
					Name:  img.Name,
					Desc:  &desc,
				})
			}
		}
	}

	if tops.Progress != nil {
		tops.Progress(transfer.Progress{
			Event: "Completed import",
		})
	}

	return nil
}

func mergeMap(m1, m2 map[string]string) map[string]string {
	merged := make(map[string]string, len(m1)+len(m2))
	for k, v := range m1 {
		merged[k] = v
	}
	for k, v := range m2 {
		merged[k] = v
	}
	return merged
}
