package metrics

import (
	"maps"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Labels represents a collection of metric label names and values.
type Labels = prometheus.Labels

// NewNamespace returns a namespace for metrics that share the given namespace
// and subsystem. labels are added as constant labels to metrics created by the
// namespace.
func NewNamespace(name, subsystem string, labels Labels) *Namespace {
	return &Namespace{
		name:      name,
		subsystem: subsystem,
		labels:    maps.Clone(labels),
	}
}

// Namespace describes a set of metrics that share a namespace and subsystem.
type Namespace struct {
	name      string
	subsystem string
	labels    prometheus.Labels
	mu        sync.Mutex
	metrics   []prometheus.Collector
}

// WithConstLabels returns a namespace with the provided set of labels merged
// with the existing constant labels on the namespace.
//
// Only metrics created with the returned namespace will get the new constant
// labels. The returned namespace must be registered separately.
func (n *Namespace) WithConstLabels(labels Labels) *Namespace {
	n.mu.Lock()
	ns := &Namespace{
		name:      n.name,
		subsystem: n.subsystem,
		labels:    mergeLabels(n.labels, labels),
	}
	n.mu.Unlock()
	return ns
}

// NewCounter creates and adds a counter to the namespace.
func (n *Namespace) NewCounter(name, help string) Counter {
	c := &counter{pc: prometheus.NewCounter(n.newCounterOpts(name, help))}
	n.Add(c)
	return c
}

// NewLabeledCounter creates and adds a counter with the given variable labels
// to the namespace.
func (n *Namespace) NewLabeledCounter(name, help string, labels ...string) LabeledCounter {
	c := &labeledCounter{pc: prometheus.NewCounterVec(n.newCounterOpts(name, help), labels)}
	n.Add(c)
	return c
}

func (n *Namespace) newCounterOpts(name, help string) prometheus.CounterOpts {
	return prometheus.CounterOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        makeName(name, Total),
		Help:        help,
		ConstLabels: n.labels,
	}
}

// NewTimer creates and adds a timer to the namespace.
func (n *Namespace) NewTimer(name, help string) Timer {
	t := &timer{
		m: prometheus.NewHistogram(n.newTimerOpts(name, help, []float64{})),
	}
	n.Add(t)
	return t
}

// NewTimerWithBuckets creates and adds a timer with the given histogram buckets
// to the namespace.
func (n *Namespace) NewTimerWithBuckets(name, help string, buckets []float64) Timer {
	t := &timer{
		m: prometheus.NewHistogram(n.newTimerOpts(name, help, buckets)),
	}
	n.Add(t)
	return t
}

// NewLabeledTimer creates and adds a timer with the given variable labels to
// the namespace.
func (n *Namespace) NewLabeledTimer(name, help string, labels ...string) LabeledTimer {
	t := &labeledTimer{
		m: prometheus.NewHistogramVec(n.newTimerOpts(name, help, []float64{}), labels),
	}
	n.Add(t)
	return t
}

// NewLabeledTimerWithBuckets creates and adds a timer with the given histogram
// buckets and variable labels to the namespace.
func (n *Namespace) NewLabeledTimerWithBuckets(name, help string, buckets []float64, labels ...string) LabeledTimer {
	t := &labeledTimer{
		m: prometheus.NewHistogramVec(n.newTimerOpts(name, help, buckets), labels),
	}
	n.Add(t)
	return t
}

func (n *Namespace) newTimerOpts(name, help string, buckets []float64) prometheus.HistogramOpts {
	opts := prometheus.HistogramOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        makeName(name, Seconds),
		Help:        help,
		ConstLabels: n.labels,
	}
	if len(buckets) > 0 {
		opts.Buckets = buckets
	}
	return opts
}

// NewGauge creates and adds a gauge with the given unit to the namespace.
func (n *Namespace) NewGauge(name, help string, unit Unit) Gauge {
	g := &gauge{
		pg: prometheus.NewGauge(n.newGaugeOpts(name, help, unit)),
	}
	n.Add(g)
	return g
}

// NewLabeledGauge creates and adds a gauge with the given unit and variable
// labels to the namespace.
func (n *Namespace) NewLabeledGauge(name, help string, unit Unit, labels ...string) LabeledGauge {
	g := &labeledGauge{
		pg: prometheus.NewGaugeVec(n.newGaugeOpts(name, help, unit), labels),
	}
	n.Add(g)
	return g
}

func (n *Namespace) newGaugeOpts(name, help string, unit Unit) prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        makeName(name, unit),
		Help:        help,
		ConstLabels: n.labels,
	}
}

// Describe sends the descriptions of all metrics in the namespace to ch.
//
// Describe implements [prometheus.Collector].
func (n *Namespace) Describe(ch chan<- *prometheus.Desc) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, metric := range n.metrics {
		metric.Describe(ch)
	}
}

// Collect sends all metrics in the namespace to ch.
//
// Collect implements [prometheus.Collector].
func (n *Namespace) Collect(ch chan<- prometheus.Metric) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, metric := range n.metrics {
		metric.Collect(ch)
	}
}

// Add adds collector to the namespace.
func (n *Namespace) Add(collector prometheus.Collector) {
	n.mu.Lock()
	n.metrics = append(n.metrics, collector)
	n.mu.Unlock()
}

// NewDesc returns a Prometheus metric descriptor using the namespace,
// subsystem, constant labels, and given variable labels.
func (n *Namespace) NewDesc(name, help string, unit Unit, labels ...string) *prometheus.Desc {
	name = makeName(name, unit)
	namespace := n.name
	if n.subsystem != "" {
		namespace += "_" + n.subsystem
	}
	name = namespace + "_" + name
	return prometheus.NewDesc(name, help, labels, n.labels)
}

// withHandlerLabel returns a copy of labels with the "handler" label set
// to handlerName.
func withHandlerLabel(labels prometheus.Labels, handlerName string) prometheus.Labels {
	out := maps.Clone(labels)
	if out == nil {
		out = make(prometheus.Labels)
	}
	out["handler"] = handlerName
	return out
}

// mergeLabels returns a new label map containing base and overrides.
// Labels in overrides take precedence over labels in base.
func mergeLabels(base prometheus.Labels, overrides Labels) prometheus.Labels {
	merged := maps.Clone(base)
	if merged == nil {
		merged = make(prometheus.Labels, len(overrides))
	}
	maps.Copy(merged, overrides)
	return merged
}

func makeName(name string, unit Unit) string {
	if unit == "" {
		return name
	}
	return name + "_" + string(unit)
}

// NewDefaultHttpMetrics creates and adds the default set of HTTP metrics for
// handlerName using the default histogram buckets.
func (n *Namespace) NewDefaultHttpMetrics(handlerName string) []*HTTPMetric {
	return n.NewHttpMetricsWithOpts(handlerName, HTTPHandlerOpts{
		DurationBuckets:     defaultDurationBuckets,
		RequestSizeBuckets:  defaultResponseSizeBuckets,
		ResponseSizeBuckets: defaultResponseSizeBuckets,
	})
}

// NewHttpMetrics creates and adds the default set of HTTP metrics for
// handlerName using the given histogram buckets.
func (n *Namespace) NewHttpMetrics(handlerName string, durationBuckets, requestSizeBuckets, responseSizeBuckets []float64) []*HTTPMetric {
	return n.NewHttpMetricsWithOpts(handlerName, HTTPHandlerOpts{
		DurationBuckets:     durationBuckets,
		RequestSizeBuckets:  requestSizeBuckets,
		ResponseSizeBuckets: responseSizeBuckets,
	})
}

// NewHttpMetricsWithOpts creates and adds the configured set of HTTP metrics
// for handlerName.
func (n *Namespace) NewHttpMetricsWithOpts(handlerName string, opts HTTPHandlerOpts) []*HTTPMetric {
	var httpMetrics []*HTTPMetric
	inFlightMetric := n.NewInFlightGaugeMetric(handlerName)
	requestTotalMetric := n.NewRequestTotalMetric(handlerName)
	requestDurationMetric := n.NewRequestDurationMetric(handlerName, opts.DurationBuckets)
	requestSizeMetric := n.NewRequestSizeMetric(handlerName, opts.RequestSizeBuckets)
	responseSizeMetric := n.NewResponseSizeMetric(handlerName, opts.ResponseSizeBuckets)
	httpMetrics = append(httpMetrics, inFlightMetric, requestDurationMetric, requestTotalMetric, requestSizeMetric, responseSizeMetric)
	return httpMetrics
}

func (n *Namespace) newHTTPMetric(collector prometheus.Collector, wrap func(http.Handler) http.Handler) *HTTPMetric {
	n.Add(collector)
	return &HTTPMetric{collector: collector, wrap: wrap}
}

// NewInFlightGaugeMetric creates and adds a metric that tracks the number of
// in-flight HTTP requests for handlerName.
func (n *Namespace) NewInFlightGaugeMetric(handlerName string) *HTTPMetric {
	collector := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        "in_flight_requests",
		Help:        "The in-flight HTTP requests",
		ConstLabels: withHandlerLabel(n.labels, handlerName),
	})
	return n.newHTTPMetric(collector, func(next http.Handler) http.Handler {
		return promhttp.InstrumentHandlerInFlight(collector, next)
	})
}

// NewRequestTotalMetric creates and adds a metric that counts HTTP requests for
// handlerName by response code and method.
func (n *Namespace) NewRequestTotalMetric(handlerName string) *HTTPMetric {
	collector := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   n.name,
			Subsystem:   n.subsystem,
			Name:        "requests_total",
			Help:        "Total number of HTTP requests made.",
			ConstLabels: withHandlerLabel(n.labels, handlerName),
		},
		[]string{"code", "method"},
	)
	return n.newHTTPMetric(collector, func(next http.Handler) http.Handler {
		return promhttp.InstrumentHandlerCounter(collector, next)
	})
}

// NewRequestDurationMetric creates and adds a metric that records HTTP request
// durations for handlerName using the given histogram buckets.
func (n *Namespace) NewRequestDurationMetric(handlerName string, buckets []float64) *HTTPMetric {
	if len(buckets) == 0 {
		panic("DurationBuckets must be provided")
	}
	opts := prometheus.HistogramOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        "request_duration_seconds",
		Help:        "The HTTP request latencies in seconds.",
		Buckets:     buckets,
		ConstLabels: withHandlerLabel(n.labels, handlerName),
	}
	collector := prometheus.NewHistogramVec(opts, []string{"method"})
	return n.newHTTPMetric(collector, func(next http.Handler) http.Handler {
		return promhttp.InstrumentHandlerDuration(collector, next)
	})
}

// NewRequestSizeMetric creates and adds a metric that records HTTP request
// sizes for handlerName using the given histogram buckets.
func (n *Namespace) NewRequestSizeMetric(handlerName string, buckets []float64) *HTTPMetric {
	if len(buckets) == 0 {
		panic("RequestSizeBuckets must be provided")
	}
	opts := prometheus.HistogramOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        "request_size_bytes",
		Help:        "The HTTP request sizes in bytes.",
		Buckets:     buckets,
		ConstLabels: withHandlerLabel(n.labels, handlerName),
	}
	collector := prometheus.NewHistogramVec(opts, []string{})
	return n.newHTTPMetric(collector, func(next http.Handler) http.Handler {
		return promhttp.InstrumentHandlerRequestSize(collector, next)
	})
}

// NewResponseSizeMetric creates and adds a metric that records HTTP response
// sizes for handlerName using the given histogram buckets.
func (n *Namespace) NewResponseSizeMetric(handlerName string, buckets []float64) *HTTPMetric {
	if len(buckets) == 0 {
		panic("ResponseSizeBuckets must be provided")
	}
	opts := prometheus.HistogramOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        "response_size_bytes",
		Help:        "The HTTP response sizes in bytes.",
		Buckets:     buckets,
		ConstLabels: withHandlerLabel(n.labels, handlerName),
	}
	collector := prometheus.NewHistogramVec(opts, []string{})
	return n.newHTTPMetric(collector, func(next http.Handler) http.Handler {
		return promhttp.InstrumentHandlerResponseSize(collector, next)
	})
}
