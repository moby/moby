package metrics

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Labels map[string]string

// NewNamespace returns a namespaces that is responsible for managing a collection of
// metrics for a particual namespace and subsystem
//
// labels allows const labels to be added to all metrics created in this namespace
// and are commonly used for data like application version and git commit
func NewNamespace(name, subsystem string, labels Labels) *Namespace {
	if labels == nil {
		labels = make(map[string]string)
	}
	return &Namespace{
		name:      name,
		subsystem: subsystem,
		labels:    labels,
	}
}

// Namespace describes a set of metrics that share a namespace and subsystem.
type Namespace struct {
	name      string
	subsystem string
	labels    Labels
	mu        sync.Mutex
	metrics   []prometheus.Collector
}

// WithConstLabels returns a namespace with the provided set of labels merged
// with the existing constant labels on the namespace.
//
//  Only metrics created with the returned namespace will get the new constant
//  labels.  The returned namespace must be registered separately.
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

func (n *Namespace) NewCounter(name, help string) Counter {
	c := &counter{pc: prometheus.NewCounter(n.newCounterOpts(name, help))}
	n.Add(c)
	return c
}

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
		ConstLabels: prometheus.Labels(n.labels),
	}
}

func (n *Namespace) NewTimer(name, help string) Timer {
	t := &timer{
		m: prometheus.NewHistogram(n.newTimerOpts(name, help)),
	}
	n.Add(t)
	return t
}

func (n *Namespace) NewLabeledTimer(name, help string, labels ...string) LabeledTimer {
	t := &labeledTimer{
		m: prometheus.NewHistogramVec(n.newTimerOpts(name, help), labels),
	}
	n.Add(t)
	return t
}

func (n *Namespace) newTimerOpts(name, help string) prometheus.HistogramOpts {
	return prometheus.HistogramOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        makeName(name, Seconds),
		Help:        help,
		ConstLabels: prometheus.Labels(n.labels),
	}
}

func (n *Namespace) NewGauge(name, help string, unit Unit) Gauge {
	g := &gauge{
		pg: prometheus.NewGauge(n.newGaugeOpts(name, help, unit)),
	}
	n.Add(g)
	return g
}

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
		ConstLabels: prometheus.Labels(n.labels),
	}
}

func (n *Namespace) Describe(ch chan<- *prometheus.Desc) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, metric := range n.metrics {
		metric.Describe(ch)
	}
}

func (n *Namespace) Collect(ch chan<- prometheus.Metric) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, metric := range n.metrics {
		metric.Collect(ch)
	}
}

func (n *Namespace) Add(collector prometheus.Collector) {
	n.mu.Lock()
	n.metrics = append(n.metrics, collector)
	n.mu.Unlock()
}

func (n *Namespace) NewDesc(name, help string, unit Unit, labels ...string) *prometheus.Desc {
	name = makeName(name, unit)
	namespace := n.name
	if n.subsystem != "" {
		namespace = fmt.Sprintf("%s_%s", namespace, n.subsystem)
	}
	name = fmt.Sprintf("%s_%s", namespace, name)
	return prometheus.NewDesc(name, help, labels, prometheus.Labels(n.labels))
}

// mergeLabels merges two or more labels objects into a single map, favoring
// the later labels.
func mergeLabels(lbs ...Labels) Labels {
	merged := make(Labels)

	for _, target := range lbs {
		for k, v := range target {
			merged[k] = v
		}
	}

	return merged
}

func makeName(name string, unit Unit) string {
	if unit == "" {
		return name
	}

	return fmt.Sprintf("%s_%s", name, unit)
}

func (n *Namespace) NewDefaultHttpMetrics(handlerName string) []*HTTPMetric {
	return n.NewHttpMetricsWithOpts(handlerName, HTTPHandlerOpts{
		DurationBuckets:     defaultDurationBuckets,
		RequestSizeBuckets:  defaultResponseSizeBuckets,
		ResponseSizeBuckets: defaultResponseSizeBuckets,
	})
}

func (n *Namespace) NewHttpMetrics(handlerName string, durationBuckets, requestSizeBuckets, responseSizeBuckets []float64) []*HTTPMetric {
	return n.NewHttpMetricsWithOpts(handlerName, HTTPHandlerOpts{
		DurationBuckets:     durationBuckets,
		RequestSizeBuckets:  requestSizeBuckets,
		ResponseSizeBuckets: responseSizeBuckets,
	})
}

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

func (n *Namespace) NewInFlightGaugeMetric(handlerName string) *HTTPMetric {
	labels := prometheus.Labels(n.labels)
	labels["handler"] = handlerName
	metric := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        "in_flight_requests",
		Help:        "The in-flight HTTP requests",
		ConstLabels: prometheus.Labels(labels),
	})
	httpMetric := &HTTPMetric{
		Collector:   metric,
		handlerType: InstrumentHandlerInFlight,
	}
	n.Add(httpMetric)
	return httpMetric
}

func (n *Namespace) NewRequestTotalMetric(handlerName string) *HTTPMetric {
	labels := prometheus.Labels(n.labels)
	labels["handler"] = handlerName
	metric := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   n.name,
			Subsystem:   n.subsystem,
			Name:        "requests_total",
			Help:        "Total number of HTTP requests made.",
			ConstLabels: prometheus.Labels(labels),
		},
		[]string{"code", "method"},
	)
	httpMetric := &HTTPMetric{
		Collector:   metric,
		handlerType: InstrumentHandlerCounter,
	}
	n.Add(httpMetric)
	return httpMetric
}
func (n *Namespace) NewRequestDurationMetric(handlerName string, buckets []float64) *HTTPMetric {
	if len(buckets) == 0 {
		panic("DurationBuckets must be provided")
	}
	labels := prometheus.Labels(n.labels)
	labels["handler"] = handlerName
	opts := prometheus.HistogramOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        "request_duration_seconds",
		Help:        "The HTTP request latencies in seconds.",
		Buckets:     buckets,
		ConstLabels: prometheus.Labels(labels),
	}
	metric := prometheus.NewHistogramVec(opts, []string{"method"})
	httpMetric := &HTTPMetric{
		Collector:   metric,
		handlerType: InstrumentHandlerDuration,
	}
	n.Add(httpMetric)
	return httpMetric
}

func (n *Namespace) NewRequestSizeMetric(handlerName string, buckets []float64) *HTTPMetric {
	if len(buckets) == 0 {
		panic("RequestSizeBuckets must be provided")
	}
	labels := prometheus.Labels(n.labels)
	labels["handler"] = handlerName
	opts := prometheus.HistogramOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        "request_size_bytes",
		Help:        "The HTTP request sizes in bytes.",
		Buckets:     buckets,
		ConstLabels: prometheus.Labels(labels),
	}
	metric := prometheus.NewHistogramVec(opts, []string{})
	httpMetric := &HTTPMetric{
		Collector:   metric,
		handlerType: InstrumentHandlerRequestSize,
	}
	n.Add(httpMetric)
	return httpMetric
}

func (n *Namespace) NewResponseSizeMetric(handlerName string, buckets []float64) *HTTPMetric {
	if len(buckets) == 0 {
		panic("ResponseSizeBuckets must be provided")
	}
	labels := prometheus.Labels(n.labels)
	labels["handler"] = handlerName
	opts := prometheus.HistogramOpts{
		Namespace:   n.name,
		Subsystem:   n.subsystem,
		Name:        "response_size_bytes",
		Help:        "The HTTP response sizes in bytes.",
		Buckets:     buckets,
		ConstLabels: prometheus.Labels(labels),
	}
	metrics := prometheus.NewHistogramVec(opts, []string{})
	httpMetric := &HTTPMetric{
		Collector:   metrics,
		handlerType: InstrumentHandlerResponseSize,
	}
	n.Add(httpMetric)
	return httpMetric
}
