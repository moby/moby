package transport // import "github.com/docker/docker/pkg/plugins/transport"

import (
	"io"
	"net/http"
)

// VersionMimetype is the Content-Type the engine sends to plugins.
const VersionMimetype = "application/vnd.docker.plugins.v1.2+json"

// RequestFactory defines an interface that
// transports can implement to create new requests.
type RequestFactory interface {
	NewRequest(path string, data io.Reader) (*http.Request, error)
}
