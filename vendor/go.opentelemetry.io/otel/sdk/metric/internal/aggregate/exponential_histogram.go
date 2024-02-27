// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const (
	expoMaxScale = 20
	expoMinScale = -10

	smallestNonZeroNormalFloat64 = 0x1p-1022

	// These redefine the Math constants with a type, so the compiler won't coerce
	// them into an int on 32 bit platforms.
	maxInt64 int64 = math.MaxInt64
	minInt64 int64 = math.MinInt64
)

// expoHistogramDataPoint is a single data point in an exponential histogram.
type expoHistogramDataPoint[N int64 | float64] struct {
	count uint64
	min   N
	max   N
	sum   N

	maxSize  int
	noMinMax bool
	noSum    bool

	scale int

	posBuckets expoBuckets
	negBuckets expoBuckets
	zeroCount  uint64
}

func newExpoHistogramDataPoint[N int64 | float64](maxSize, maxScale int, noMinMax, noSum bool) *expoHistogramDataPoint[N] {
	f := math.MaxFloat64
	max := N(f) // if N is int64, max will overflow to -9223372036854775808
	min := N(-f)
	if N(maxInt64) > N(f) {
		max = N(maxInt64)
		min = N(minInt64)
	}
	return &expoHistogramDataPoint[N]{
		min:      max,
		max:      min,
		maxSize:  maxSize,
		noMinMax: noMinMax,
		noSum:    noSum,
		scale:    maxScale,
	}
}

// record adds a new measurement to the histogram. It will rescale the buckets if needed.
func (p *expoHistogramDataPoint[N]) record(v N) {
	p.count++

	if !p.noMinMax {
		if v < p.min {
			p.min = v
		}
		if v > p.max {
			p.max = v
		}
	}
	if !p.noSum {
		p.sum += v
	}

	absV := math.Abs(float64(v))

	if float64(absV) == 0.0 {
		p.zeroCount++
		return
	}

	bin := p.getBin(absV)

	bucket := &p.posBuckets
	if v < 0 {
		bucket = &p.negBuckets
	}

	// If the new bin would make the counts larger than maxScale, we need to
	// downscale current measurements.
	if scaleDelta := p.scaleChange(bin, bucket.startBin, len(bucket.counts)); scaleDelta > 0 {
		if p.scale-scaleDelta < expoMinScale {
			// With a scale of -10 there is only two buckets for the whole range of float64 values.
			// This can only happen if there is a max size of 1.
			otel.Handle(errors.New("exponential histogram scale underflow"))
			return
		}
		// Downscale
		p.scale -= scaleDelta
		p.posBuckets.downscale(scaleDelta)
		p.negBuckets.downscale(scaleDelta)

		bin = p.getBin(absV)
	}

	bucket.record(bin)
}

// getBin returns the bin v should be recorded into.
func (p *expoHistogramDataPoint[N]) getBin(v float64) int {
	frac, exp := math.Frexp(v)
	if p.scale <= 0 {
		// Because of the choice of fraction is always 1 power of two higher than we want.
		correction := 1
		if frac == .5 {
			// If v is an exact power of two the frac will be .5 and the exp
			// will be one higher than we want.
			correction = 2
		}
		return (exp - correction) >> (-p.scale)
	}
	return exp<<p.scale + int(math.Log(frac)*scaleFactors[p.scale]) - 1
}

// scaleFactors are constants used in calculating the logarithm index. They are
// equivalent to 2^index/log(2).
var scaleFactors = [21]float64{
	math.Ldexp(math.Log2E, 0),
	math.Ldexp(math.Log2E, 1),
	math.Ldexp(math.Log2E, 2),
	math.Ldexp(math.Log2E, 3),
	math.Ldexp(math.Log2E, 4),
	math.Ldexp(math.Log2E, 5),
	math.Ldexp(math.Log2E, 6),
	math.Ldexp(math.Log2E, 7),
	math.Ldexp(math.Log2E, 8),
	math.Ldexp(math.Log2E, 9),
	math.Ldexp(math.Log2E, 10),
	math.Ldexp(math.Log2E, 11),
	math.Ldexp(math.Log2E, 12),
	math.Ldexp(math.Log2E, 13),
	math.Ldexp(math.Log2E, 14),
	math.Ldexp(math.Log2E, 15),
	math.Ldexp(math.Log2E, 16),
	math.Ldexp(math.Log2E, 17),
	math.Ldexp(math.Log2E, 18),
	math.Ldexp(math.Log2E, 19),
	math.Ldexp(math.Log2E, 20),
}

// scaleChange returns the magnitude of the scale change needed to fit bin in
// the bucket. If no scale change is needed 0 is returned.
func (p *expoHistogramDataPoint[N]) scaleChange(bin, startBin, length int) int {
	if length == 0 {
		// No need to rescale if there are no buckets.
		return 0
	}

	low := startBin
	high := bin
	if startBin >= bin {
		low = bin
		high = startBin + length - 1
	}

	count := 0
	for high-low >= p.maxSize {
		low = low >> 1
		high = high >> 1
		count++
		if count > expoMaxScale-expoMinScale {
			return count
		}
	}
	return count
}

// expoBuckets is a set of buckets in an exponential histogram.
type expoBuckets struct {
	startBin int
	counts   []uint64
}

// record increments the count for the given bin, and expands the buckets if needed.
// Size changes must be done before calling this function.
func (b *expoBuckets) record(bin int) {
	if len(b.counts) == 0 {
		b.counts = []uint64{1}
		b.startBin = bin
		return
	}

	endBin := b.startBin + len(b.counts) - 1

	// if the new bin is inside the current range
	if bin >= b.startBin && bin <= endBin {
		b.counts[bin-b.startBin]++
		return
	}
	// if the new bin is before the current start add spaces to the counts
	if bin < b.startBin {
		origLen := len(b.counts)
		newLength := endBin - bin + 1
		shift := b.startBin - bin

		if newLength > cap(b.counts) {
			b.counts = append(b.counts, make([]uint64, newLength-len(b.counts))...)
		}

		copy(b.counts[shift:origLen+shift], b.counts[:])
		b.counts = b.counts[:newLength]
		for i := 1; i < shift; i++ {
			b.counts[i] = 0
		}
		b.startBin = bin
		b.counts[0] = 1
		return
	}
	// if the new is after the end add spaces to the end
	if bin > endBin {
		if bin-b.startBin < cap(b.counts) {
			b.counts = b.counts[:bin-b.startBin+1]
			for i := endBin + 1 - b.startBin; i < len(b.counts); i++ {
				b.counts[i] = 0
			}
			b.counts[bin-b.startBin] = 1
			return
		}

		end := make([]uint64, bin-b.startBin-len(b.counts)+1)
		b.counts = append(b.counts, end...)
		b.counts[bin-b.startBin] = 1
	}
}

// downscale shrinks a bucket by a factor of 2*s. It will sum counts into the
// correct lower resolution bucket.
func (b *expoBuckets) downscale(delta int) {
	// Example
	// delta = 2
	// Original offset: -6
	// Counts: [ 3,  1,  2,  3,  4,  5, 6, 7, 8, 9, 10]
	// bins:    -6  -5, -4, -3, -2, -1, 0, 1, 2, 3, 4
	// new bins:-2, -2, -1, -1, -1, -1, 0, 0, 0, 0, 1
	// new Offset: -2
	// new Counts: [4, 14, 30, 10]

	if len(b.counts) <= 1 || delta < 1 {
		b.startBin = b.startBin >> delta
		return
	}

	steps := 1 << delta
	offset := b.startBin % steps
	offset = (offset + steps) % steps // to make offset positive
	for i := 1; i < len(b.counts); i++ {
		idx := i + offset
		if idx%steps == 0 {
			b.counts[idx/steps] = b.counts[i]
			continue
		}
		b.counts[idx/steps] += b.counts[i]
	}

	lastIdx := (len(b.counts) - 1 + offset) / steps
	b.counts = b.counts[:lastIdx+1]
	b.startBin = b.startBin >> delta
}

// newExponentialHistogram returns an Aggregator that summarizes a set of
// measurements as an exponential histogram. Each histogram is scoped by attributes
// and the aggregation cycle the measurements were made in.
func newExponentialHistogram[N int64 | float64](maxSize, maxScale int32, noMinMax, noSum bool) *expoHistogram[N] {
	return &expoHistogram[N]{
		noSum:    noSum,
		noMinMax: noMinMax,
		maxSize:  int(maxSize),
		maxScale: int(maxScale),

		values: make(map[attribute.Set]*expoHistogramDataPoint[N]),

		start: now(),
	}
}

// expoHistogram summarizes a set of measurements as an histogram with exponentially
// defined buckets.
type expoHistogram[N int64 | float64] struct {
	noSum    bool
	noMinMax bool
	maxSize  int
	maxScale int

	values   map[attribute.Set]*expoHistogramDataPoint[N]
	valuesMu sync.Mutex

	start time.Time
}

func (e *expoHistogram[N]) measure(_ context.Context, value N, attr attribute.Set) {
	// Ignore NaN and infinity.
	if math.IsInf(float64(value), 0) || math.IsNaN(float64(value)) {
		return
	}

	e.valuesMu.Lock()
	defer e.valuesMu.Unlock()

	v, ok := e.values[attr]
	if !ok {
		v = newExpoHistogramDataPoint[N](e.maxSize, e.maxScale, e.noMinMax, e.noSum)
		e.values[attr] = v
	}
	v.record(value)
}

func (e *expoHistogram[N]) delta(dest *metricdata.Aggregation) int {
	t := now()

	// If *dest is not a metricdata.ExponentialHistogram, memory reuse is missed.
	// In that case, use the zero-value h and hope for better alignment next cycle.
	h, _ := (*dest).(metricdata.ExponentialHistogram[N])
	h.Temporality = metricdata.DeltaTemporality

	e.valuesMu.Lock()
	defer e.valuesMu.Unlock()

	n := len(e.values)
	hDPts := reset(h.DataPoints, n, n)

	var i int
	for a, b := range e.values {
		hDPts[i].Attributes = a
		hDPts[i].StartTime = e.start
		hDPts[i].Time = t
		hDPts[i].Count = b.count
		hDPts[i].Scale = int32(b.scale)
		hDPts[i].ZeroCount = b.zeroCount
		hDPts[i].ZeroThreshold = 0.0

		hDPts[i].PositiveBucket.Offset = int32(b.posBuckets.startBin)
		hDPts[i].PositiveBucket.Counts = reset(hDPts[i].PositiveBucket.Counts, len(b.posBuckets.counts), len(b.posBuckets.counts))
		copy(hDPts[i].PositiveBucket.Counts, b.posBuckets.counts)

		hDPts[i].NegativeBucket.Offset = int32(b.negBuckets.startBin)
		hDPts[i].NegativeBucket.Counts = reset(hDPts[i].NegativeBucket.Counts, len(b.negBuckets.counts), len(b.negBuckets.counts))

		if !e.noSum {
			hDPts[i].Sum = b.sum
		}
		if !e.noMinMax {
			hDPts[i].Min = metricdata.NewExtrema(b.min)
			hDPts[i].Max = metricdata.NewExtrema(b.max)
		}

		delete(e.values, a)
		i++
	}
	e.start = t
	h.DataPoints = hDPts
	*dest = h
	return n
}

func (e *expoHistogram[N]) cumulative(dest *metricdata.Aggregation) int {
	t := now()

	// If *dest is not a metricdata.ExponentialHistogram, memory reuse is missed.
	// In that case, use the zero-value h and hope for better alignment next cycle.
	h, _ := (*dest).(metricdata.ExponentialHistogram[N])
	h.Temporality = metricdata.CumulativeTemporality

	e.valuesMu.Lock()
	defer e.valuesMu.Unlock()

	n := len(e.values)
	hDPts := reset(h.DataPoints, n, n)

	var i int
	for a, b := range e.values {
		hDPts[i].Attributes = a
		hDPts[i].StartTime = e.start
		hDPts[i].Time = t
		hDPts[i].Count = b.count
		hDPts[i].Scale = int32(b.scale)
		hDPts[i].ZeroCount = b.zeroCount
		hDPts[i].ZeroThreshold = 0.0

		hDPts[i].PositiveBucket.Offset = int32(b.posBuckets.startBin)
		hDPts[i].PositiveBucket.Counts = reset(hDPts[i].PositiveBucket.Counts, len(b.posBuckets.counts), len(b.posBuckets.counts))
		copy(hDPts[i].PositiveBucket.Counts, b.posBuckets.counts)

		hDPts[i].NegativeBucket.Offset = int32(b.negBuckets.startBin)
		hDPts[i].NegativeBucket.Counts = reset(hDPts[i].NegativeBucket.Counts, len(b.negBuckets.counts), len(b.negBuckets.counts))

		if !e.noSum {
			hDPts[i].Sum = b.sum
		}
		if !e.noMinMax {
			hDPts[i].Min = metricdata.NewExtrema(b.min)
			hDPts[i].Max = metricdata.NewExtrema(b.max)
		}

		i++
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
	}

	h.DataPoints = hDPts
	*dest = h
	return n
}
