package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// StartTimer begins a timer observation at the callsite. When the target
// operation is completed, the caller should call the return done func().
func StartTimer(timer Timer) (done func()) {
	start := time.Now()
	return func() {
		timer.Update(time.Since(start))
	}
}

// Timer is a metric that allows collecting the duration of an action in seconds
type Timer interface {
	// Update records an observation, duration, and converts to the target
	// units.
	Update(duration time.Duration)

	// UpdateSince will add the duration from the provided starting time to the
	// timer's summary with the precisions that was used in creation of the timer
	UpdateSince(time.Time)
}

// LabeledTimer is a timer that must have label values populated before use.
type LabeledTimer interface {
	WithValues(labels ...string) *labeledTimerObserver
}

type labeledTimer struct {
	m *prometheus.HistogramVec
}

type labeledTimerObserver struct {
	m prometheus.Observer
}

func (lbo *labeledTimerObserver) Update(duration time.Duration) {
	lbo.m.Observe(duration.Seconds())
}

func (lbo *labeledTimerObserver) UpdateSince(since time.Time) {
	lbo.m.Observe(time.Since(since).Seconds())
}

func (lt *labeledTimer) WithValues(labels ...string) *labeledTimerObserver {
	return &labeledTimerObserver{m: lt.m.WithLabelValues(labels...)}
}

func (lt *labeledTimer) Describe(c chan<- *prometheus.Desc) {
	lt.m.Describe(c)
}

func (lt *labeledTimer) Collect(c chan<- prometheus.Metric) {
	lt.m.Collect(c)
}

type timer struct {
	m prometheus.Observer
}

func (t *timer) Update(duration time.Duration) {
	t.m.Observe(duration.Seconds())
}

func (t *timer) UpdateSince(since time.Time) {
	t.m.Observe(time.Since(since).Seconds())
}

func (t *timer) Describe(c chan<- *prometheus.Desc) {
	c <- t.m.(prometheus.Metric).Desc()
}

func (t *timer) Collect(c chan<- prometheus.Metric) {
	// Are there any observers that don't implement Collector? It is really
	// unclear what the point of the upstream change was, but we'll let this
	// panic if we get an observer that doesn't implement collector. In this
	// case, we should almost always see metricVec objects, so this should
	// never panic.
	t.m.(prometheus.Collector).Collect(c)
}
