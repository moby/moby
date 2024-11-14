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
	"io"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Resolver provides remotes based on a locator.
type Resolver interface {
	// Resolve attempts to resolve the reference into a name and descriptor.
	//
	// The argument `ref` should be a scheme-less URI representing the remote.
	// Structurally, it has a host and path. The "host" can be used to directly
	// reference a specific host or be matched against a specific handler.
	//
	// The returned name should be used to identify the referenced entity.
	// Depending on the remote namespace, this may be immutable or mutable.
	// While the name may differ from ref, it should itself be a valid ref.
	//
	// If the resolution fails, an error will be returned.
	Resolve(ctx context.Context, ref string) (name string, desc ocispec.Descriptor, err error)

	// Fetcher returns a new fetcher for the provided reference.
	// All content fetched from the returned fetcher will be
	// from the namespace referred to by ref.
	Fetcher(ctx context.Context, ref string) (Fetcher, error)

	// Pusher returns a new pusher for the provided reference
	// The returned Pusher should satisfy content.Ingester and concurrent attempts
	// to push the same blob using the Ingester API should result in ErrUnavailable.
	Pusher(ctx context.Context, ref string) (Pusher, error)
}

// Fetcher fetches content.
// A fetcher implementation may implement the FetcherByDigest interface too.
type Fetcher interface {
	// Fetch the resource identified by the descriptor.
	Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error)
}

// FetcherByDigest fetches content by the digest.
type FetcherByDigest interface {
	// FetchByDigest fetches the resource identified by the digest.
	//
	// FetcherByDigest usually returns an incomplete descriptor.
	// Typically, the media type is always set to "application/octet-stream",
	// and the annotations are unset.
	FetchByDigest(ctx context.Context, dgst digest.Digest, opts ...FetchByDigestOpts) (io.ReadCloser, ocispec.Descriptor, error)
}

// Pusher pushes content
type Pusher interface {
	// Push returns a content writer for the given resource identified
	// by the descriptor.
	Push(ctx context.Context, d ocispec.Descriptor) (content.Writer, error)
}

// FetcherFunc allows package users to implement a Fetcher with just a
// function.
type FetcherFunc func(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error)

// Fetch content
func (fn FetcherFunc) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	return fn(ctx, desc)
}

// PusherFunc allows package users to implement a Pusher with just a
// function.
type PusherFunc func(ctx context.Context, desc ocispec.Descriptor) (content.Writer, error)

// Push content
func (fn PusherFunc) Push(ctx context.Context, desc ocispec.Descriptor) (content.Writer, error) {
	return fn(ctx, desc)
}

// FetchByDigestConfig provides configuration for fetching content by digest
type FetchByDigestConfig struct {
	//Mediatype specifies mediatype header to append for fetch request
	Mediatype string
}

// FetchByDigestOpts allows callers to set options for fetch object
type FetchByDigestOpts func(context.Context, *FetchByDigestConfig) error

// WithMediaType sets the media type header for fetch request
func WithMediaType(mediatype string) FetchByDigestOpts {
	return func(ctx context.Context, cfg *FetchByDigestConfig) error {
		cfg.Mediatype = mediatype
		return nil
	}
}
