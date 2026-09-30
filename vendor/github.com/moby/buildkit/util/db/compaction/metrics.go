package compaction

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/moby/buildkit/util/db"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics shares instruments across databases using the daemon's meter provider.
type Metrics struct {
	stateDir    string
	attempts    metric.Int64Counter
	duration    metric.Float64Histogram
	reclaimed   metric.Int64Counter
	size        metric.Int64ObservableGauge
	reclaimable metric.Int64ObservableGauge
	age         metric.Float64ObservableGauge
	pending     metric.Float64ObservableGauge
	mu          sync.Mutex
	databases   map[*databaseMetrics]struct{}
}

type databaseMetrics struct {
	owner    *Metrics
	path     string
	stats    db.CompactionStats
	observed time.Time
	pending  [2]time.Time
}

func NewMetrics(mp metric.MeterProvider, stateDir string) (*Metrics, error) {
	if mp == nil {
		return nil, nil
	}
	m := &Metrics{stateDir: stateDir, databases: make(map[*databaseMetrics]struct{})}
	meter := mp.Meter("github.com/moby/buildkit/util/db/compaction")
	var err error
	m.attempts, err = meter.Int64Counter("buildkit.compaction.attempts", metric.WithDescription("Compaction attempts by database path, trigger, and outcome."))
	if err != nil {
		return nil, err
	}
	m.duration, err = meter.Float64Histogram("buildkit.compaction.copy.duration", metric.WithUnit("s"), metric.WithDescription("Copy and replacement duration, including canceled and failed copies."))
	if err != nil {
		return nil, err
	}
	m.reclaimed, err = meter.Int64Counter("buildkit.compaction.reclaimed", metric.WithUnit("By"), metric.WithDescription("Bytes reclaimed by successful database replacements."))
	if err != nil {
		return nil, err
	}
	m.size, err = meter.Int64ObservableGauge("buildkit.compaction.database.size", metric.WithUnit("By"), metric.WithDescription("Sum of last observed file sizes by database path."))
	if err != nil {
		return nil, err
	}
	m.reclaimable, err = meter.Int64ObservableGauge("buildkit.compaction.database.reclaimable", metric.WithUnit("By"), metric.WithDescription("Sum of last estimated reclaimable bytes by database path."))
	if err != nil {
		return nil, err
	}
	m.age, err = meter.Float64ObservableGauge("buildkit.compaction.database.observation.age", metric.WithUnit("s"), metric.WithDescription("Age of the oldest size observation contributing to each database path."))
	if err != nil {
		return nil, err
	}
	m.pending, err = meter.Float64ObservableGauge("buildkit.compaction.pending.duration", metric.WithUnit("s"), metric.WithDescription("Longest current wait for an attempt by database path and trigger."))
	if err != nil {
		return nil, err
	}
	_, err = meter.RegisterCallback(m.collect, m.size, m.reclaimable, m.age, m.pending)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Metrics) attach(path string) *databaseMetrics {
	if m == nil {
		return nil
	}
	if path != "" {
		relative, err := filepath.Rel(m.stateDir, path)
		if err == nil && filepath.IsLocal(relative) {
			path = filepath.ToSlash(relative)
		} else {
			path = ""
		}
	}
	d := &databaseMetrics{owner: m, path: path}
	m.mu.Lock()
	m.databases[d] = struct{}{}
	m.mu.Unlock()
	return d
}

func (d *databaseMetrics) close() {
	if d == nil {
		return
	}
	d.owner.mu.Lock()
	delete(d.owner.databases, d)
	d.owner.mu.Unlock()
}

func (d *databaseMetrics) observe(stats db.CompactionStats, at time.Time) {
	if d == nil {
		return
	}
	d.owner.mu.Lock()
	if !at.Before(d.observed) {
		d.stats, d.observed = stats, at
	}
	d.owner.mu.Unlock()
}

func (d *databaseMetrics) wait(manual, pending bool) {
	if d == nil {
		return
	}
	i := 0
	if manual {
		i = 1
	}
	d.owner.mu.Lock()
	if pending {
		if d.pending[i].IsZero() {
			d.pending[i] = time.Now()
		}
	} else {
		d.pending[i] = time.Time{}
	}
	d.owner.mu.Unlock()
}

func (d *databaseMetrics) record(ctx context.Context, manual bool, res db.CompactResult, err, cause error) {
	if d == nil {
		return
	}
	trigger := "automatic"
	if manual {
		trigger = "manual"
	}
	outcome := "skipped"
	switch {
	case !res.Compacted && cause != nil:
		outcome = "canceled"
	case err != nil:
		outcome = "failed"
	case res.Compacted:
		outcome = "completed"
	}
	attrs := []attribute.KeyValue{attribute.String("database.path", d.path), attribute.String("trigger", trigger)}
	d.owner.attempts.Add(ctx, 1, metric.WithAttributes(append(attrs, attribute.String("outcome", outcome))...))
	if res.Duration > 0 {
		d.owner.duration.Record(ctx, res.Duration.Seconds(), metric.WithAttributes(append(attrs, attribute.String("outcome", outcome))...))
	}
	if res.Compacted && res.SizeAfter > 0 && res.SizeBefore > res.SizeAfter {
		d.owner.reclaimed.Add(ctx, res.SizeBefore-res.SizeAfter, metric.WithAttributes(attrs...))
	}
}

func (m *Metrics) collect(_ context.Context, observer metric.Observer) error {
	type snapshot struct {
		size, reclaimable int64
		observed          time.Time
		pending           [2]time.Time
	}
	groups := map[string]snapshot{}
	m.mu.Lock()
	for d := range m.databases {
		g := groups[d.path]
		if !d.observed.IsZero() {
			g.size += d.stats.Size
			g.reclaimable += d.stats.Reclaimable
			if g.observed.IsZero() || d.observed.Before(g.observed) {
				g.observed = d.observed
			}
		}
		for i, at := range d.pending {
			if !at.IsZero() && (g.pending[i].IsZero() || at.Before(g.pending[i])) {
				g.pending[i] = at
			}
		}
		groups[d.path] = g
	}
	m.mu.Unlock()
	now := time.Now()
	for path, g := range groups {
		attrs := metric.WithAttributes(attribute.String("database.path", path))
		if !g.observed.IsZero() {
			observer.ObserveInt64(m.size, g.size, attrs)
			observer.ObserveInt64(m.reclaimable, g.reclaimable, attrs)
			observer.ObserveFloat64(m.age, max(now.Sub(g.observed).Seconds(), 0), attrs)
		}
		for i, trigger := range []string{"automatic", "manual"} {
			age := float64(0)
			if !g.pending[i].IsZero() {
				age = max(now.Sub(g.pending[i]).Seconds(), 0)
			}
			observer.ObserveFloat64(m.pending, age, metric.WithAttributes(attribute.String("database.path", path), attribute.String("trigger", trigger)))
		}
	}
	return nil
}

func (s *Scheduler) stats() (db.CompactionStats, error) {
	if s.metrics == nil {
		return s.backend.CompactionStats()
	}
	at := time.Now()
	stats, err := s.backend.CompactionStats()
	if err == nil {
		s.metrics.observe(stats, at)
	}
	return stats, err
}

func (s *Scheduler) recordAttempt(ctx context.Context, manual bool, res db.CompactResult, err, cause error) {
	if s.metrics == nil {
		return
	}
	s.metrics.record(ctx, manual, res, err, cause)
	_, _ = s.stats()
}
