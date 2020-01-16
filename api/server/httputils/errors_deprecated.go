package httputils // import "github.com/moby/moby/api/server/httputils"
import "github.com/moby/moby/errdefs"

// GetHTTPErrorStatusCode retrieves status code from error message.
//
// Deprecated: use errdefs.GetHTTPErrorStatusCode
func GetHTTPErrorStatusCode(err error) int {
	return errdefs.GetHTTPErrorStatusCode(err)
}
