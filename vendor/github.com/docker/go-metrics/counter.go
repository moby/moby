package metrics

import "github.com/prometheus/client_golang/prometheus"

// Counter is a metrics that can only increment its current count
type Counter interface {
	// Inc adds Sum(vs) to the counter. Sum(vs) must be positive.
	//
	// If len(vs) == 0, increments the counter by 1.
	Inc(vs ...float64)
}

// LabeledCounter is counter that must have labels populated before use.
type LabeledCounter interface {
	WithValues(vs ...string) Counter
}

type labeledCounter struct {
	pc *prometheus.CounterVec
}

func (lc *labeledCounter) WithValues(vs ...string) Counter {
	return &counter{pc: lc.pc.WithLabelValues(vs...)}
}

func (lc *labeledCounter) Describe(ch chan<- *prometheus.Desc) {
	lc.pc.Describe(ch)
}

func (lc *labeledCounter) Collect(ch chan<- prometheus.Metric) {
	lc.pc.Collect(ch)
}

type counter struct {
	pc prometheus.Counter
}

func (c *counter) Inc(vs ...float64) {
	if len(vs) == 0 {
		c.pc.Inc()
	}

	c.pc.Add(sumFloat64(vs...))
}

func (c *counter) Describe(ch chan<- *prometheus.Desc) {
	c.pc.Describe(ch)
}

func (c *counter) Collect(ch chan<- prometheus.Metric) {
	c.pc.Collect(ch)
}
