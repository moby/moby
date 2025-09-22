package opts

import (
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ImageHistoryOptions struct {
	ApiOptions ImageHistoryApiOptions
}

type ImageHistoryApiOptions struct {
	// Platform selects the specific platform of a multi-platform image to inspect.
	//
	// This option is only available for API version 1.49 and up.
	Platform *ocispec.Platform
}
