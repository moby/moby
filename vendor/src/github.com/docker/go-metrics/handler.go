package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

// Handler returns the global http.Handler that provides the prometheus
// metrics format on GET requests
func Handler() http.Handler {
	return prometheus.Handler()
}
