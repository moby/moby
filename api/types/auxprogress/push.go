package auxprogress

import (
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ManifestPushedInsteadOfIndex is a note that is sent when a manifest is pushed
// instead of an index.  It is sent when the pushed image is an multi-platform
// index, but the whole index couldn't be pushed.
type ManifestPushedInsteadOfIndex struct {
	ManifestPushedInsteadOfIndex bool `json:"manifestPushedInsteadOfIndex"` // Always true

	// OriginalIndex is the descriptor of the original image index.
	OriginalIndex ocispec.Descriptor `json:"originalIndex"`

	// SelectedManifest is the descriptor of the manifest that was pushed instead.
	SelectedManifest ocispec.Descriptor `json:"selectedManifest"`
}

// ContentMissing is a note that is sent when push fails because the content is missing.
type ContentMissing struct {
	ContentMissing bool `json:"contentMissing"` // Always true

	// Desc is the descriptor of the root object that was attempted to be pushed.
	Desc ocispec.Descriptor `json:"desc"`
}
