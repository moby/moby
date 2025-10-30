package client

import (
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageImportSource holds source information for ImageImport
type ImageImportSource struct {
	Source     io.Reader // Source is the data to send to the server to create this image from. You must set SourceName to "-" to leverage this.
	SourceName string    // SourceName is the name of the image to pull. Set to "-" to leverage the Source attribute.
}

// ImageImportOptions holds information to import images from the client host.
type ImageImportOptions struct {
	Tag      string           // Tag is the name to tag this image with. This attribute is deprecated.
	Message  string           // Message is the message to tag the image with
	Changes  []string         // Changes are the raw changes to apply to this image
	Platform ocispec.Platform // Platform is the target platform of the image
}

// ImageImportResult holds the response body returned by the daemon for image import.
type ImageImportResult struct {
	rc io.ReadCloser
}

func (r ImageImportResult) Read(p []byte) (n int, err error) {
	if r.rc == nil {
		return 0, io.EOF
	}
	return r.rc.Read(p)
}

func (r ImageImportResult) Close() error {
	if r.rc == nil {
		return nil
	}
	return r.rc.Close()
}
