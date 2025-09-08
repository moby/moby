package imagepush

import (
	"context"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Option interface {
	ApplyImagePushOption(*InternalOptions) error
}
type imagePushOptionFunc func(opt *InternalOptions) error

func (f imagePushOptionFunc) ApplyImagePushOption(o *InternalOptions) error {
	return f(o)
}

// WithPrivilegeFunc sets a function that clients can supply to retry operations
// after getting an authorization error. This function returns the registry
// authentication header value in base64 encoded format, or an error if the
// privilege request fails.
//
// For details, refer to [github.com/moby/moby/api/types/registry.RequestAuthConfig].
func WithPrivilegeFunc(fn func(context.Context) (string, error)) Option {
	return imagePushOptionFunc(func(opt *InternalOptions) error {
		opt.PrivilegeFunc = fn
		return nil
	})
}

func WithAllTags() Option {
	return imagePushOptionFunc(func(opt *InternalOptions) error {
		opt.All = true
		return nil
	})
}

func WithRegistryAuth(auth string) Option {
	return imagePushOptionFunc(func(opt *InternalOptions) error {
		opt.RegistryAuth = auth
		return nil
	})
}

type InternalOptions struct {
	PrivilegeFunc func(context.Context) (string, error)

	All          bool
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry

	// Platform is an optional field that selects a specific platform to push
	// when the image is a multi-platform image.
	// Using this will only push a single platform-specific manifest.
	Platform *ocispec.Platform `json:",omitempty"`
}
