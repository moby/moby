package client

import (
	"context"
	"io"
)

// ImportSource holds source information for ImageImport
type ImportSource struct {
	Source     io.Reader // Source is the data to send to the server to create this image from. You must set SourceName to "-" to leverage this.
	SourceName string    // SourceName is the name of the image to pull. Set to "-" to leverage the Source attribute.
}

// ImportOptions holds information to import images from the client host.
type ImportOptions struct {
	Tag      string   // Tag is the name to tag this image with. This attribute is deprecated.
	Message  string   // Message is the message to tag the image with
	Changes  []string // Changes are the raw changes to apply to this image
	Platform string   // Platform is the target platform of the image
}

// PullOptions holds information to pull images.
type PullOptions struct {
	All          bool
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry

	// PrivilegeFunc is a function that clients can supply to retry operations
	// after getting an authorization error. This function returns the registry
	// authentication header value in base64 encoded format, or an error if the
	// privilege request fails.
	//
	// For details, refer to [github.com/moby/moby/api/types/registry.RequestAuthConfig].
	PrivilegeFunc func(context.Context) (string, error)
	Platform      string
}
