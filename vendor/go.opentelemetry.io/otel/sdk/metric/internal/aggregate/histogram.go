// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregate

import (
	"context"
	"slices"
	"sort"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/internal/x"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// histogramPoint is a single histogram point, used in delta aggregations.
type histogramPoint[N int64 | float64] struct {
	attrs         attribute.Set
	res           FilteredExemplarReservoir[N]
	dropExemplars bool
	histogramPointCounters[N]
}

// hotColdHistogramPoint a hot and cold histogram points, used in cumulative
// aggregations.
type hotColdHistogramPoint[N int64 | float64] struct {
	hcwg         hotColdWaitGroup
	hotColdPoint [2]histogramPointCounters[N]

	attrs         attribute.Set
	res           FilteredExemplarReservoir[N]
	startTime     time.Time
	dropExemplars bool
}

// histogramPointCounters contains only the atomic counter data, and is used by
// both histogramPoint and hotColdHistogramPoint.
type histogramPointCounters[N int64 | float64] struct {
	counts []atomic.Uint64
	total  atomicCounter[N]
	minMax atomicMinMax[N]
}

func (b *histogramPointCounters[N]) loadCountsInto(into *[]uint64) uint64 {
	counts := reset(*into, len(b.counts), len(b.counts))
	count := uint64(0)
	for i := range b.counts {
		c := b.counts[i].Load()
		counts[i] = c
		count += c
	}
	*into = counts
	return count
}

// mergeIntoAndReset merges this set of histogram counter data into another,
// and resets the state of this set of counters. This is used by
// hotColdHistogramPoint to ensure that the cumulative counters continue to
// accumulate after being read.
func (b *histogramPointCounters[N]) mergeIntoAndReset( // nolint:revive // Intentional internal control flag
	into *histogramPointCounters[N],
	noMinMax, noSum bool,
) {
	for i := range b.counts {
		into.counts[i].Add(b.counts[i].Load())
		b.counts[i].Store(0)
	}

	if !noMinMax {
		// Do not reset min or max because cumulative min and max only ever grow
		// smaller or larger respectively.

		if b.minMax.set.Load() {
			into.minMax.Update(b.minMax.minimum.Load())
			into.minMax.Update(b.minMax.maximum.Load())
		}
	}
	if !noSum {
		into.total.add(b.total.load())
		b.total.reset()
	}
}

// deltaHistogram is a histogram whose internal storage is reset when it is
// collected.
//
// deltaHistogram's measure is implemented without locking, even when called
// concurrently with collect. This is done by maintaining two separate maps:
// one "hot" which is concurrently updated by measure(), and one "cold", which
// is read and reset by collect(). The [hotcoldWaitGroup] allows collect() to
// swap the hot and cold maps, and wait for updates to the cold map to complete
// prior to reading. deltaHistogram swaps ald clears complete maps so that
// unused attribute sets do not report in subsequent collect() calls.
type deltaHistogram[N int64 | float64] struct {
	hcwg          hotColdWaitGroup
	hotColdValMap [2]limitedSyncMap[*histogramPoint[N]]

	start    time.Time
	noMinMax bool
	noSum    bool
	bounds   []float64
	newRes   func(attribute.Set) FilteredExemplarReservoir[N]
}

func (s *deltaHistogram[N]) measure(
	ctx context.Context,
	value N,
	fltrAttr attribute.Set,
	droppedAttr []attribute.KeyValue,
) {
	hotIdx := s.hcwg.start()
	defer s.hcwg.done(hotIdx)
	h := s.hotColdValMap[hotIdx].LoadOrStoreAttr(fltrAttr, func(attr attribute.Set) *histogramPoint[N] {
		r := s.newRes(attr)
		_, isDrop := r.(*dropRes[N])
		hPt := &histogramPoint[N]{
			res:           r,
			attrs:         attr,
			dropExemplars: isDrop,
			// N+1 buckets. For example:
			//
			//   bounds = [0, 5, 10]
			//
			// Then,
			//
			//   counts = (-∞, 0], (0, 5.0], (5.0, 10.0], (10.0, +∞)
			histogramPointCounters: histogramPointCounters[N]{counts: make([]atomic.Uint64, len(s.bounds)+1)},
		}
		return hPt
	})

	// This search will return an index in the range [0, len(s.bounds)], where
	// it will return len(s.bounds) if value is greater than the last element
	// of s.bounds. This aligns with the histogramPoint in that the length of histogramPoint
	// is len(s.bounds)+1, with the last bucket representing:
	// (s.bounds[len(s.bounds)-1], +∞).
	idx := sort.SearchFloat64s(s.bounds, float64(value))
	h.counts[idx].Add(1)
	if !s.noMinMax {
		h.minMax.Update(value)
	}
	if !s.noSum {
		h.total.add(value)
	}
	if !h.dropExemplars {
		h.res.Offer(ctx, value, droppedAttr)
	}
}

// newDeltaHistogram returns a histogram that is reset each time it is
// collected.
func newDeltaHistogram[N int64 | float64](
	boundaries []float64,
	noMinMax, noSum bool,
	limit int,
	r func(attribute.Set) FilteredExemplarReservoir[N],
) *deltaHistogram[N] {
	// The responsibility of keeping all histogramPoint correctly associated with the
	// passed boundaries is ultimately this type's responsibility. Make a copy
	// here so we can always guarantee this. Or, in the case of failure, have
	// complete control over the fix.
	b := slices.Clone(boundaries)
	slices.Sort(b)
	return &deltaHistogram[N]{
		start:    now(),
		noMinMax: noMinMax,
		noSum:    noSum,
		bounds:   b,
		newRes:   r,
		hotColdValMap: [2]limitedSyncMap[*histogramPoint[N]]{
			{aggLimit: limit},
			{aggLimit: limit},
		},
	}
}

func (s *deltaHistogram[N]) collect(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.Histogram, memory reuse is missed. In that
	// case, use the zero-value h and hope for better alignment next cycle.
	h, _ := (*dest).(metricdata.Histogram[N])
	h.Temporality = metricdata.DeltaTemporality

	// delta always clears values on collection
	readIdx := s.hcwg.swapHotAndWait()

	// Do not allow modification of our copy of bounds.
	bounds := slices.Clone(s.bounds)

	// The len will not change while we iterate over values, since we waited
	// for all writes to finish to the cold values and len.
	n := s.hotColdValMap[readIdx].Len()
	hDPts := reset(h.DataPoints, n, n)

	var i int
	s.hotColdValMap[readIdx].Range(func(_, value any) bool {
		val := value.(*histogramPoint[N])

		count := val.loadCountsInto(&hDPts[i].BucketCounts)
		hDPts[i].Attributes = val.attrs
		hDPts[i].StartTime = s.start
		hDPts[i].Time = t
		hDPts[i].Count = count
		hDPts[i].Bounds = bounds

		if !s.noSum {
			hDPts[i].Sum = val.total.load()
		} else {
			hDPts[i].Sum = 0
		}

		if !s.noMinMax && val.minMax.set.Load() {
			hDPts[i].Min = metricdata.NewExtrema(val.minMax.minimum.Load())
			hDPts[i].Max = metricdata.NewExtrema(val.minMax.maximum.Load())
		} else {
			hDPts[i].Min = metricdata.Extrema[N]{}
			hDPts[i].Max = metricdata.Extrema[N]{}
		}

		collectExemplars(&hDPts[i].Exemplars, val.res.Collect)

		i++
		return true
	})
	// Unused attribute sets do not report.
	s.hotColdValMap[readIdx].Clear()
	// The delta collection cycle resets.
	s.start = t

	h.DataPoints = hDPts
	*dest = h

	return n
}

// cumulativeHistogram summarizes a set of measurements as an histogram with explicitly
// defined histogramPoint.
//
// cumulativeHistogram's measure is implemented without locking, even when
// called concurrently with collect. This is done by maintaining two separate
// histogramPointCounters for each attribute set: one "hot" which is
// concurrently updated by measure(), and one "cold", which is read and reset
// by collect(). The [hotcoldWaitGroup] allows collect() to swap the hot and
// cold counters, and wait for updates to the cold counters to complete prior
// to reading. Unlike deltaHistogram, this maintains a single map so that the
// preserved attribute sets do not change when collect() is called.
type cumulativeHistogram[N int64 | float64] struct {
	values limitedSyncMap[*hotColdHistogramPoint[N]]

	start    time.Time
	noMinMax bool
	noSum    bool
	bounds   []float64
	newRes   func(attribute.Set) FilteredExemplarReservoir[N]
}

// newCumulativeHistogram returns a histogram that accumulates measurements
// into a histogram data structure. It is never reset.
func newCumulativeHistogram[N int64 | float64](
	boundaries []float64,
	noMinMax, noSum bool,
	limit int,
	r func(attribute.Set) FilteredExemplarReservoir[N],
) *cumulativeHistogram[N] {
	// The responsibility of keeping all histogramPoint correctly associated with the
	// passed boundaries is ultimately this type's responsibility. Make a copy
	// here so we can always guarantee this. Or, in the case of failure, have
	// complete control over the fix.
	b := slices.Clone(boundaries)
	slices.Sort(b)
	return &cumulativeHistogram[N]{
		start:    now(),
		noMinMax: noMinMax,
		noSum:    noSum,
		bounds:   b,
		newRes:   r,
		values:   limitedSyncMap[*hotColdHistogramPoint[N]]{aggLimit: limit},
	}
}

func (s *cumulativeHistogram[N]) measure(
	ctx context.Context,
	value N,
	fltrAttr attribute.Set,
	droppedAttr []attribute.KeyValue,
) {
	h := s.values.LoadOrStoreAttr(fltrAttr, func(attr attribute.Set) *hotColdHistogramPoint[N] {
		r := s.newRes(attr)
		_, isDrop := r.(*dropRes[N])
		hPt := &hotColdHistogramPoint[N]{
			res:           r,
			attrs:         attr,
			startTime:     now(),
			dropExemplars: isDrop,
			// N+1 buckets. For example:
			//
			//   bounds = [0, 5, 10]
			//
			// Then,
			//
			//   count = (-∞, 0], (0, 5.0], (5.0, 10.0], (10.0, +∞)
			hotColdPoint: [2]histogramPointCounters[N]{
				{
					counts: make([]atomic.Uint64, len(s.bounds)+1),
				},
				{
					counts: make([]atomic.Uint64, len(s.bounds)+1),
				},
			},
		}
		return hPt
	})

	// This search will return an index in the range [0, len(s.bounds)], where
	// it will return len(s.bounds) if value is greater than the last element
	// of s.bounds. This aligns with the histogramPoint in that the length of histogramPoint
	// is len(s.bounds)+1, with the last bucket representing:
	// (s.bounds[len(s.bounds)-1], +∞).
	idx := sort.SearchFloat64s(s.bounds, float64(value))

	hotIdx := h.hcwg.start()
	defer h.hcwg.done(hotIdx)

	h.hotColdPoint[hotIdx].counts[idx].Add(1)
	if !s.noMinMax {
		h.hotColdPoint[hotIdx].minMax.Update(value)
	}
	if !s.noSum {
		h.hotColdPoint[hotIdx].total.add(value)
	}
	if !h.dropExemplars {
		h.res.Offer(ctx, value, droppedAttr)
	}
}

func (s *cumulativeHistogram[N]) collect(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.Histogram, memory reuse is missed. In that
	// case, use the zero-value h and hope for better alignment next cycle.
	h, _ := (*dest).(metricdata.Histogram[N])
	h.Temporality = metricdata.CumulativeTemporality

	// Do not allow modification of our copy of bounds.
	bounds := slices.Clone(s.bounds)

	// Pre-size hDPts so per-series destination slots are addressable by index.
	// This lets loadCountsInto and collectExemplars reuse each slot's existing
	// BucketCounts/Exemplars slices from the previous cycle. s.values is
	// append-only here during cumulative collect, and limitedSyncMap only
	// increments its len after a successful underlying LoadOrStore, so Range
	// will visit at least n entries. Concurrent measurers may add more between
	// Len() and Range; those extra slots are appended on demand inside the
	// loop.
	n := s.values.Len()
	hDPts := reset(h.DataPoints, n, n)

	perSeriesStartTimeEnabled := x.PerSeriesStartTimestamps.Enabled()

	var i int
	s.values.Range(func(_, value any) bool {
		val := value.(*hotColdHistogramPoint[N])

		startTime := s.start
		if perSeriesStartTimeEnabled {
			startTime = val.startTime
		}
		// swap, observe, and clear the point
		readIdx := val.hcwg.swapHotAndWait()

		// Concurrent writers can grow s.values between Len() and Range.
		// Cold-path grow.
		if i >= len(hDPts) {
			hDPts = append(hDPts, metricdata.HistogramDataPoint[N]{})
		}
		dp := &hDPts[i]

		count := val.hotColdPoint[readIdx].loadCountsInto(&dp.BucketCounts)
		dp.Attributes = val.attrs
		dp.StartTime = startTime
		dp.Time = t
		dp.Count = count
		dp.Bounds = bounds
		if !s.noSum {
			dp.Sum = val.hotColdPoint[readIdx].total.load()
		} else {
			// Slot may be reused; clear stale Sum from previous series.
			var zero N
			dp.Sum = zero
		}
		if !s.noMinMax && val.hotColdPoint[readIdx].minMax.set.Load() {
			dp.Min = metricdata.NewExtrema(val.hotColdPoint[readIdx].minMax.minimum.Load())
			dp.Max = metricdata.NewExtrema(val.hotColdPoint[readIdx].minMax.maximum.Load())
		} else {
			// Slot may be reused; clear stale Min/Max from previous series.
			dp.Min = metricdata.Extrema[N]{}
			dp.Max = metricdata.Extrema[N]{}
		}
		// Once we've read the point, merge it back into the hot histogram
		// point since it is cumulative.
		hotIdx := (readIdx + 1) % 2
		val.hotColdPoint[readIdx].mergeIntoAndReset(&val.hotColdPoint[hotIdx], s.noMinMax, s.noSum)

		collectExemplars(&dp.Exemplars, val.res.Collect)

		i++
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
		return true
	})

	h.DataPoints = hDPts
	*dest = h

	return i
}
