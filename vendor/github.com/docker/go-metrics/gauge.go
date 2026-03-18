package metrics

import "github.com/prometheus/client_golang/prometheus"

// Gauge is a metric that allows incrementing and decrementing a value
type Gauge interface {
	Inc(...float64)
	Dec(...float64)

	// Add adds the provided value to the gauge's current value
	Add(float64)

	// Set replaces the gauge's current value with the provided value
	Set(float64)
}

// LabeledGauge describes a gauge the must have values populated before use.
type LabeledGauge interface {
	WithValues(labels ...string) Gauge
}

type labeledGauge struct {
	pg *prometheus.GaugeVec
}

func (lg *labeledGauge) WithValues(labels ...string) Gauge {
	return &gauge{pg: lg.pg.WithLabelValues(labels...)}
}

func (lg *labeledGauge) Describe(c chan<- *prometheus.Desc) {
	lg.pg.Describe(c)
}

func (lg *labeledGauge) Collect(c chan<- prometheus.Metric) {
	lg.pg.Collect(c)
}

type gauge struct {
	pg prometheus.Gauge
}

func (g *gauge) Inc(vs ...float64) {
	if len(vs) == 0 {
		g.pg.Inc()
	}

	g.Add(sumFloat64(vs...))
}

func (g *gauge) Dec(vs ...float64) {
	if len(vs) == 0 {
		g.pg.Dec()
	}

	g.Add(-sumFloat64(vs...))
}

func (g *gauge) Add(v float64) {
	g.pg.Add(v)
}

func (g *gauge) Set(v float64) {
	g.pg.Set(v)
}

func (g *gauge) Describe(c chan<- *prometheus.Desc) {
	g.pg.Describe(c)
}

func (g *gauge) Collect(c chan<- prometheus.Metric) {
	g.pg.Collect(c)
}
