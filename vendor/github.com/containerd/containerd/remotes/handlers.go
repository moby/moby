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

package remotes

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

type refKeyPrefix struct{}

// WithMediaTypeKeyPrefix adds a custom key prefix for a media type which is used when storing
// data in the content store from the FetchHandler.
//
// Used in `MakeRefKey` to determine what the key prefix should be.
func WithMediaTypeKeyPrefix(ctx context.Context, mediaType, prefix string) context.Context {
	var values map[string]string
	if v := ctx.Value(refKeyPrefix{}); v != nil {
		values = v.(map[string]string)
	} else {
		values = make(map[string]string)
	}

	values[mediaType] = prefix
	return context.WithValue(ctx, refKeyPrefix{}, values)
}

// MakeRefKey returns a unique reference for the descriptor. This reference can be
// used to lookup ongoing processes related to the descriptor. This function
// may look to the context to namespace the reference appropriately.
func MakeRefKey(ctx context.Context, desc ocispec.Descriptor) string {
	key := desc.Digest.String()
	if desc.Annotations != nil {
		if name, ok := desc.Annotations[ocispec.AnnotationRefName]; ok {
			key = fmt.Sprintf("%s@%s", name, desc.Digest.String())
		}
	}

	if v := ctx.Value(refKeyPrefix{}); v != nil {
		values := v.(map[string]string)
		if prefix := values[desc.MediaType]; prefix != "" {
			return prefix + "-" + key
		}
	}

	switch mt := desc.MediaType; {
	case mt == images.MediaTypeDockerSchema2Manifest || mt == ocispec.MediaTypeImageManifest:
		return "manifest-" + key
	case mt == images.MediaTypeDockerSchema2ManifestList || mt == ocispec.MediaTypeImageIndex:
		return "index-" + key
	case images.IsLayerType(mt):
		return "layer-" + key
	case images.IsKnownConfig(mt):
		return "config-" + key
	default:
		log.G(ctx).Warnf("reference for unknown type: %s", mt)
		return "unknown-" + key
	}
}

// FetchHandler returns a handler that will fetch all content into the ingester
// discovered in a call to Dispatch. Use with ChildrenHandler to do a full
// recursive fetch.
func FetchHandler(ingester content.Ingester, fetcher Fetcher) images.HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) (subdescs []ocispec.Descriptor, err error) {
		ctx = log.WithLogger(ctx, log.G(ctx).WithFields(logrus.Fields{
			"digest":    desc.Digest,
			"mediatype": desc.MediaType,
			"size":      desc.Size,
		}))

		switch desc.MediaType {
		case images.MediaTypeDockerSchema1Manifest:
			return nil, fmt.Errorf("%v not supported", desc.MediaType)
		default:
			err := fetch(ctx, ingester, fetcher, desc)
			return nil, err
		}
	}
}

func fetch(ctx context.Context, ingester content.Ingester, fetcher Fetcher, desc ocispec.Descriptor) error {
	log.G(ctx).Debug("fetch")

	cw, err := content.OpenWriter(ctx, ingester, content.WithRef(MakeRefKey(ctx, desc)), content.WithDescriptor(desc))
	if err != nil {
		if errdefs.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	defer cw.Close()

	ws, err := cw.Status()
	if err != nil {
		return err
	}

	if desc.Size == 0 {
		// most likely a poorly configured registry/web front end which responded with no
		// Content-Length header; unable (not to mention useless) to commit a 0-length entry
		// into the content store. Error out here otherwise the error sent back is confusing
		return errors.Wrapf(errdefs.ErrInvalidArgument, "unable to fetch descriptor (%s) which reports content size of zero", desc.Digest)
	}
	if ws.Offset == desc.Size {
		// If writer is already complete, commit and return
		err := cw.Commit(ctx, desc.Size, desc.Digest)
		if err != nil && !errdefs.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed commit on ref %q", ws.Ref)
		}
		return nil
	}

	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return err
	}
	defer rc.Close()

	return content.Copy(ctx, cw, rc, desc.Size, desc.Digest)
}

// PushHandler returns a handler that will push all content from the provider
// using a writer from the pusher.
func PushHandler(pusher Pusher, provider content.Provider) images.HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		ctx = log.WithLogger(ctx, log.G(ctx).WithFields(logrus.Fields{
			"digest":    desc.Digest,
			"mediatype": desc.MediaType,
			"size":      desc.Size,
		}))

		err := push(ctx, provider, pusher, desc)
		return nil, err
	}
}

func push(ctx context.Context, provider content.Provider, pusher Pusher, desc ocispec.Descriptor) error {
	log.G(ctx).Debug("push")

	var (
		cw  content.Writer
		err error
	)
	if cs, ok := pusher.(content.Ingester); ok {
		cw, err = content.OpenWriter(ctx, cs, content.WithRef(MakeRefKey(ctx, desc)), content.WithDescriptor(desc))
	} else {
		cw, err = pusher.Push(ctx, desc)
	}
	if err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return err
		}

		return nil
	}
	defer cw.Close()

	ra, err := provider.ReaderAt(ctx, desc)
	if err != nil {
		return err
	}
	defer ra.Close()

	rd := io.NewSectionReader(ra, 0, desc.Size)
	return content.Copy(ctx, cw, rd, desc.Size, desc.Digest)
}

// PushContent pushes content specified by the descriptor from the provider.
//
// Base handlers can be provided which will be called before any push specific
// handlers.
func PushContent(ctx context.Context, pusher Pusher, desc ocispec.Descriptor, store content.Store, limiter *semaphore.Weighted, platform platforms.MatchComparer, wrapper func(h images.Handler) images.Handler) error {

	var m sync.Mutex
	manifestStack := []ocispec.Descriptor{}

	filterHandler := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest,
			images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
			m.Lock()
			manifestStack = append(manifestStack, desc)
			m.Unlock()
			return nil, images.ErrStopHandler
		default:
			return nil, nil
		}
	})

	pushHandler := PushHandler(pusher, store)

	platformFilterhandler := images.FilterPlatforms(images.ChildrenHandler(store), platform)

	annotateHandler := annotateDistributionSourceHandler(platformFilterhandler, store)

	var handler images.Handler = images.Handlers(
		annotateHandler,
		filterHandler,
		pushHandler,
	)
	if wrapper != nil {
		handler = wrapper(handler)
	}

	if err := images.Dispatch(ctx, handler, limiter, desc); err != nil {
		return err
	}

	// Iterate in reverse order as seen, parent always uploaded after child
	for i := len(manifestStack) - 1; i >= 0; i-- {
		_, err := pushHandler(ctx, manifestStack[i])
		if err != nil {
			// TODO(estesp): until we have a more complete method for index push, we need to report
			// missing dependencies in an index/manifest list by sensing the "400 Bad Request"
			// as a marker for this problem
			if (manifestStack[i].MediaType == ocispec.MediaTypeImageIndex ||
				manifestStack[i].MediaType == images.MediaTypeDockerSchema2ManifestList) &&
				errors.Cause(err) != nil && strings.Contains(errors.Cause(err).Error(), "400 Bad Request") {
				return errors.Wrap(err, "manifest list/index references to blobs and/or manifests are missing in your target registry")
			}
			return err
		}
	}

	return nil
}

// FilterManifestByPlatformHandler allows Handler to handle non-target
// platform's manifest and configuration data.
func FilterManifestByPlatformHandler(f images.HandlerFunc, m platforms.Matcher) images.HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		children, err := f(ctx, desc)
		if err != nil {
			return nil, err
		}

		// no platform information
		if desc.Platform == nil || m == nil {
			return children, nil
		}

		var descs []ocispec.Descriptor
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
			if m.Match(*desc.Platform) {
				descs = children
			} else {
				for _, child := range children {
					if child.MediaType == images.MediaTypeDockerSchema2Config ||
						child.MediaType == ocispec.MediaTypeImageConfig {

						descs = append(descs, child)
					}
				}
			}
		default:
			descs = children
		}
		return descs, nil
	}
}

// annotateDistributionSourceHandler add distribution source label into
// annotation of config or blob descriptor.
func annotateDistributionSourceHandler(f images.HandlerFunc, manager content.Manager) images.HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		children, err := f(ctx, desc)
		if err != nil {
			return nil, err
		}

		// only add distribution source for the config or blob data descriptor
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest,
			images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
		default:
			return children, nil
		}

		for i := range children {
			child := children[i]

			info, err := manager.Info(ctx, child.Digest)
			if err != nil {
				return nil, err
			}

			for k, v := range info.Labels {
				if !strings.HasPrefix(k, "containerd.io/distribution.source.") {
					continue
				}

				if child.Annotations == nil {
					child.Annotations = map[string]string{}
				}
				child.Annotations[k] = v
			}

			children[i] = child
		}
		return children, nil
	}
}
