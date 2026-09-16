package metrics

import "github.com/prometheus/client_golang/prometheus"

// Register is a convenience wrapper around [prometheus.MustRegister] that
// registers all metrics in n with the default Prometheus registry.
func Register(n *Namespace) {
	prometheus.MustRegister(n)
}

// Deregister is a convenience wrapper around [prometheus.Unregister] that
// unregisters all metrics in n from the default Prometheus registry.
func Deregister(n *Namespace) {
	prometheus.Unregister(n)
}
