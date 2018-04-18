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

package images

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var (
	// ErrSkipDesc is used to skip processing of a descriptor and
	// its descendants.
	ErrSkipDesc = fmt.Errorf("skip descriptor")

	// ErrStopHandler is used to signify that the descriptor
	// has been handled and should not be handled further.
	// This applies only to a single descriptor in a handler
	// chain and does not apply to descendant descriptors.
	ErrStopHandler = fmt.Errorf("stop handler")
)

// Handler handles image manifests
type Handler interface {
	Handle(ctx context.Context, desc ocispec.Descriptor) (subdescs []ocispec.Descriptor, err error)
}

// HandlerFunc function implementing the Handler interface
type HandlerFunc func(ctx context.Context, desc ocispec.Descriptor) (subdescs []ocispec.Descriptor, err error)

// Handle image manifests
func (fn HandlerFunc) Handle(ctx context.Context, desc ocispec.Descriptor) (subdescs []ocispec.Descriptor, err error) {
	return fn(ctx, desc)
}

// Handlers returns a handler that will run the handlers in sequence.
//
// A handler may return `ErrStopHandler` to stop calling additional handlers
func Handlers(handlers ...Handler) HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) (subdescs []ocispec.Descriptor, err error) {
		var children []ocispec.Descriptor
		for _, handler := range handlers {
			ch, err := handler.Handle(ctx, desc)
			if err != nil {
				if errors.Cause(err) == ErrStopHandler {
					break
				}
				return nil, err
			}

			children = append(children, ch...)
		}

		return children, nil
	}
}

// Walk the resources of an image and call the handler for each. If the handler
// decodes the sub-resources for each image,
//
// This differs from dispatch in that each sibling resource is considered
// synchronously.
func Walk(ctx context.Context, handler Handler, descs ...ocispec.Descriptor) error {
	for _, desc := range descs {

		children, err := handler.Handle(ctx, desc)
		if err != nil {
			if errors.Cause(err) == ErrSkipDesc {
				continue // don't traverse the children.
			}
			return err
		}

		if len(children) > 0 {
			if err := Walk(ctx, handler, children...); err != nil {
				return err
			}
		}
	}

	return nil
}

// Dispatch runs the provided handler for content specified by the descriptors.
// If the handler decode subresources, they will be visited, as well.
//
// Handlers for siblings are run in parallel on the provided descriptors. A
// handler may return `ErrSkipDesc` to signal to the dispatcher to not traverse
// any children.
//
// Typically, this function will be used with `FetchHandler`, often composed
// with other handlers.
//
// If any handler returns an error, the dispatch session will be canceled.
func Dispatch(ctx context.Context, handler Handler, descs ...ocispec.Descriptor) error {
	eg, ctx := errgroup.WithContext(ctx)
	for _, desc := range descs {
		desc := desc

		eg.Go(func() error {
			desc := desc

			children, err := handler.Handle(ctx, desc)
			if err != nil {
				if errors.Cause(err) == ErrSkipDesc {
					return nil // don't traverse the children.
				}
				return err
			}

			if len(children) > 0 {
				return Dispatch(ctx, handler, children...)
			}

			return nil
		})
	}

	return eg.Wait()
}

// ChildrenHandler decodes well-known manifest types and returns their children.
//
// This is useful for supporting recursive fetch and other use cases where you
// want to do a full walk of resources.
//
// One can also replace this with another implementation to allow descending of
// arbitrary types.
func ChildrenHandler(provider content.Provider) HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		return Children(ctx, provider, desc)
	}
}

// SetChildrenLabels is a handler wrapper which sets labels for the content on
// the children returned by the handler and passes through the children.
// Must follow a handler that returns the children to be labeled.
func SetChildrenLabels(manager content.Manager, f HandlerFunc) HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		children, err := f(ctx, desc)
		if err != nil {
			return children, err
		}

		if len(children) > 0 {
			info := content.Info{
				Digest: desc.Digest,
				Labels: map[string]string{},
			}
			fields := []string{}
			for i, ch := range children {
				info.Labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i)] = ch.Digest.String()
				fields = append(fields, fmt.Sprintf("labels.containerd.io/gc.ref.content.%d", i))
			}

			_, err := manager.Update(ctx, info, fields...)
			if err != nil {
				return nil, err
			}
		}

		return children, err
	}
}

// FilterPlatforms is a handler wrapper which limits the descriptors returned
// by a handler to the specified platforms.
func FilterPlatforms(f HandlerFunc, platformList ...string) HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		children, err := f(ctx, desc)
		if err != nil {
			return children, err
		}

		var descs []ocispec.Descriptor

		if len(platformList) == 0 {
			descs = children
		} else {
			for _, platform := range platformList {
				p, err := platforms.Parse(platform)
				if err != nil {
					return nil, err
				}
				matcher := platforms.NewMatcher(p)

				for _, d := range children {
					if d.Platform == nil || matcher.Match(*d.Platform) {
						descs = append(descs, d)
					}
				}
			}
		}

		return descs, nil
	}
}
