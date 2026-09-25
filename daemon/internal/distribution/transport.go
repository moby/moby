package distribution

import (
	"net/http"

	"github.com/docker/distribution/registry/client/transport"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// newTransport creates a new transport which will apply modifiers to
// the request on a RoundTrip call.
func newTransport(base http.RoundTripper, modifiers ...transport.RequestModifier) http.RoundTripper {
	tr := transport.NewTransport(base, modifiers...)

	// Wrap the transport with OpenTelemetry instrumentation
	// This propagates the Traceparent header.
	return otelhttp.NewTransport(tr)
}
