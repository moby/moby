package remotecontext // import "github.com/docker/docker/builder/remotecontext"

import (
	"mime"
	"net/http"
)

// MIME content types.
const (
	mimeTypeTextPlain   = "text/plain"
	mimeTypeOctetStream = "application/octet-stream"
)

// detectContentType returns a best guess representation of the MIME
// content type for the bytes at c.  The value detected by
// http.DetectContentType is guaranteed not be nil, defaulting to
// application/octet-stream when a better guess cannot be made. The
// result of this detection is then run through mime.ParseMediaType()
// which separates the actual MIME string from any parameters.
func detectContentType(c []byte) (string, error) {
	contentType, _, err := mime.ParseMediaType(http.DetectContentType(c))
	if err != nil {
		return "", err
	}
	return contentType, nil
}
