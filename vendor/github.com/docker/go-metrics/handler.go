package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPHandlerOpts describes a set of configurable options of http metrics
type HTTPHandlerOpts struct {
	DurationBuckets     []float64
	RequestSizeBuckets  []float64
	ResponseSizeBuckets []float64
}

const (
	InstrumentHandlerResponseSize = iota
	InstrumentHandlerRequestSize
	InstrumentHandlerDuration
	InstrumentHandlerCounter
	InstrumentHandlerInFlight
)

type HTTPMetric struct {
	prometheus.Collector
	handlerType int
}

var (
	defaultDurationBuckets     = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 25, 60}
	defaultRequestSizeBuckets  = prometheus.ExponentialBuckets(1024, 2, 22) //1K to 4G
	defaultResponseSizeBuckets = defaultRequestSizeBuckets
)

// Handler returns the global http.Handler that provides the prometheus
// metrics format on GET requests. This handler is no longer instrumented.
func Handler() http.Handler {
	return promhttp.Handler()
}

func InstrumentHandler(metrics []*HTTPMetric, handler http.Handler) http.HandlerFunc {
	return InstrumentHandlerFunc(metrics, handler.ServeHTTP)
}

func InstrumentHandlerFunc(metrics []*HTTPMetric, handlerFunc http.HandlerFunc) http.HandlerFunc {
	var handler http.Handler
	handler = http.HandlerFunc(handlerFunc)
	for _, metric := range metrics {
		switch metric.handlerType {
		case InstrumentHandlerResponseSize:
			if collector, ok := metric.Collector.(prometheus.ObserverVec); ok {
				handler = promhttp.InstrumentHandlerResponseSize(collector, handler)
			}
		case InstrumentHandlerRequestSize:
			if collector, ok := metric.Collector.(prometheus.ObserverVec); ok {
				handler = promhttp.InstrumentHandlerRequestSize(collector, handler)
			}
		case InstrumentHandlerDuration:
			if collector, ok := metric.Collector.(prometheus.ObserverVec); ok {
				handler = promhttp.InstrumentHandlerDuration(collector, handler)
			}
		case InstrumentHandlerCounter:
			if collector, ok := metric.Collector.(*prometheus.CounterVec); ok {
				handler = promhttp.InstrumentHandlerCounter(collector, handler)
			}
		case InstrumentHandlerInFlight:
			if collector, ok := metric.Collector.(prometheus.Gauge); ok {
				handler = promhttp.InstrumentHandlerInFlight(collector, handler)
			}
		}
	}
	return handler.ServeHTTP
}
