// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metric // import "go.opentelemetry.io/otel/sdk/metric"

import (
	"os"
	"runtime"
	"slices"

	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/internal/aggregate"
)

// reservoirFunc returns the appropriately configured exemplar reservoir
// creation func based on the passed InstrumentKind and user defined
// environment variables.
//
// Note: This will only return non-nil values when the experimental exemplar
// feature is enabled and the OTEL_METRICS_EXEMPLAR_FILTER environment variable
// is not set to always_off.
func reservoirFunc[N int64 | float64](agg Aggregation) func() aggregate.FilteredExemplarReservoir[N] {
	// https://github.com/open-telemetry/opentelemetry-specification/blob/d4b241f451674e8f611bb589477680341006ad2b/specification/configuration/sdk-environment-variables.md#exemplar
	const filterEnvKey = "OTEL_METRICS_EXEMPLAR_FILTER"

	var filter exemplar.Filter

	switch os.Getenv(filterEnvKey) {
	case "always_on":
		filter = exemplar.AlwaysOnFilter
	case "always_off":
		return aggregate.DropReservoir
	case "trace_based":
		fallthrough
	default:
		filter = exemplar.TraceBasedFilter
	}

	// https://github.com/open-telemetry/opentelemetry-specification/blob/d4b241f451674e8f611bb589477680341006ad2b/specification/metrics/sdk.md#exemplar-defaults
	// Explicit bucket histogram aggregation with more than 1 bucket will
	// use AlignedHistogramBucketExemplarReservoir.
	a, ok := agg.(AggregationExplicitBucketHistogram)
	if ok && len(a.Boundaries) > 0 {
		cp := slices.Clone(a.Boundaries)
		return func() aggregate.FilteredExemplarReservoir[N] {
			bounds := cp
			return aggregate.NewFilteredExemplarReservoir[N](filter, exemplar.NewHistogramReservoir(bounds))
		}
	}

	var n int
	if a, ok := agg.(AggregationBase2ExponentialHistogram); ok {
		// Base2 Exponential Histogram Aggregation SHOULD use a
		// SimpleFixedSizeExemplarReservoir with a reservoir equal to the
		// smaller of the maximum number of buckets configured on the
		// aggregation or twenty (e.g. min(20, max_buckets)).
		n = int(a.MaxSize)
		if n > 20 {
			n = 20
		}
	} else {
		// https://github.com/open-telemetry/opentelemetry-specification/blob/e94af89e3d0c01de30127a0f423e912f6cda7bed/specification/metrics/sdk.md#simplefixedsizeexemplarreservoir
		//   This Exemplar reservoir MAY take a configuration parameter for
		//   the size of the reservoir. If no size configuration is
		//   provided, the default size MAY be the number of possible
		//   concurrent threads (e.g. number of CPUs) to help reduce
		//   contention. Otherwise, a default size of 1 SHOULD be used.
		n = runtime.NumCPU()
		if n < 1 {
			// Should never be the case, but be defensive.
			n = 1
		}
	}

	return func() aggregate.FilteredExemplarReservoir[N] {
		return aggregate.NewFilteredExemplarReservoir[N](filter, exemplar.NewFixedSizeReservoir(n))
	}
}
