package image // import "github.com/docker/docker/api/types/image"

import (
	"io"
	"time"
)

// Metadata contains engine-local data about the image
type Metadata struct {
	LastTagTime time.Time `json:",omitempty"`
}

// PushResult contains the tag, manifest digest, and manifest size from the
// push. It's used to signal this information to the trust code in the client
// so it can sign the manifest if necessary.
type PushResult struct {
	Tag    string
	Digest string
	Size   int
}

// BuildResult contains the image id of a successful build
type BuildResult struct {
	ID string
}

// ImageBuildResponse holds information
// returned by a server after building
// an image.
type BuildResponse struct {
	Body   io.ReadCloser
	OSType string
}

// ImageImportSource holds source information for ImageImport
type ImportSource struct {
	Source     io.Reader // Source is the data to send to the server to create this image from. You must set SourceName to "-" to leverage this.
	SourceName string    // SourceName is the name of the image to pull. Set to "-" to leverage the Source attribute.
}

// LoadResponse returns information to the client about a load process.
type LoadResponse struct {
	// Body must be closed to avoid a resource leak
	Body io.ReadCloser
	JSON bool
}
