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
