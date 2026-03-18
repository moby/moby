// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package internal provides internal functionality for the opencensus package.
package internal // import "go.opentelemetry.io/otel/bridge/opencensus/internal/ocmetric"

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"

	ocmetricdata "go.opencensus.io/metric/metricdata"
	octrace "go.opencensus.io/trace"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var (
	errAggregationType              = errors.New("unsupported OpenCensus aggregation type")
	errMismatchedValueTypes         = errors.New("wrong value type for data point")
	errNegativeCount                = errors.New("distribution or summary count is negative")
	errNegativeBucketCount          = errors.New("distribution bucket count is negative")
	errMismatchedAttributeKeyValues = errors.New("mismatched number of attribute keys and values")
	errInvalidExemplarSpanContext   = errors.New(
		"span context exemplar attachment does not contain an OpenCensus SpanContext",
	)
)

// ConvertMetrics converts metric data from OpenCensus to OpenTelemetry.
func ConvertMetrics(ocmetrics []*ocmetricdata.Metric) ([]metricdata.Metrics, error) {
	otelMetrics := make([]metricdata.Metrics, 0, len(ocmetrics))
	var err error
	for _, ocm := range ocmetrics {
		if ocm == nil {
			continue
		}
		agg, aggregationErr := convertAggregation(ocm)
		if aggregationErr != nil {
			err = errors.Join(err, fmt.Errorf("error converting metric %v: %w", ocm.Descriptor.Name, aggregationErr))
			continue
		}
		otelMetrics = append(otelMetrics, metricdata.Metrics{
			Name:        ocm.Descriptor.Name,
			Description: ocm.Descriptor.Description,
			Unit:        string(ocm.Descriptor.Unit),
			Data:        agg,
		})
	}
	if err != nil {
		return otelMetrics, fmt.Errorf("error converting from OpenCensus to OpenTelemetry: %w", err)
	}
	return otelMetrics, nil
}

// convertAggregation produces an aggregation based on the OpenCensus Metric.
func convertAggregation(metric *ocmetricdata.Metric) (metricdata.Aggregation, error) {
	labelKeys := metric.Descriptor.LabelKeys
	switch metric.Descriptor.Type {
	case ocmetricdata.TypeGaugeInt64:
		return convertGauge[int64](labelKeys, metric.TimeSeries)
	case ocmetricdata.TypeGaugeFloat64:
		return convertGauge[float64](labelKeys, metric.TimeSeries)
	case ocmetricdata.TypeCumulativeInt64:
		return convertSum[int64](labelKeys, metric.TimeSeries)
	case ocmetricdata.TypeCumulativeFloat64:
		return convertSum[float64](labelKeys, metric.TimeSeries)
	case ocmetricdata.TypeCumulativeDistribution:
		return convertHistogram(labelKeys, metric.TimeSeries)
	case ocmetricdata.TypeSummary:
		return convertSummary(labelKeys, metric.TimeSeries)
	}
	return nil, fmt.Errorf("%w: %q", errAggregationType, metric.Descriptor.Type)
}

// convertGauge converts an OpenCensus gauge to an OpenTelemetry gauge aggregation.
func convertGauge[N int64 | float64](
	labelKeys []ocmetricdata.LabelKey,
	ts []*ocmetricdata.TimeSeries,
) (metricdata.Gauge[N], error) {
	points, err := convertNumberDataPoints[N](labelKeys, ts)
	return metricdata.Gauge[N]{DataPoints: points}, err
}

// convertSum converts an OpenCensus cumulative to an OpenTelemetry sum aggregation.
func convertSum[N int64 | float64](
	labelKeys []ocmetricdata.LabelKey,
	ts []*ocmetricdata.TimeSeries,
) (metricdata.Sum[N], error) {
	points, err := convertNumberDataPoints[N](labelKeys, ts)
	// OpenCensus sums are always Cumulative
	return metricdata.Sum[N]{DataPoints: points, Temporality: metricdata.CumulativeTemporality, IsMonotonic: true}, err
}

// convertNumberDataPoints converts OpenCensus TimeSeries to OpenTelemetry DataPoints.
func convertNumberDataPoints[N int64 | float64](
	labelKeys []ocmetricdata.LabelKey,
	ts []*ocmetricdata.TimeSeries,
) ([]metricdata.DataPoint[N], error) {
	var points []metricdata.DataPoint[N]
	var err error
	for _, t := range ts {
		attrs, attrsErr := convertAttrs(labelKeys, t.LabelValues)
		if attrsErr != nil {
			err = errors.Join(err, attrsErr)
			continue
		}
		for _, p := range t.Points {
			v, ok := p.Value.(N)
			if !ok {
				err = errors.Join(err, fmt.Errorf("%w: %q", errMismatchedValueTypes, p.Value))
				continue
			}
			points = append(points, metricdata.DataPoint[N]{
				Attributes: attrs,
				StartTime:  t.StartTime,
				Time:       p.Time,
				Value:      v,
			})
		}
	}
	return points, err
}

// convertHistogram converts OpenCensus Distribution timeseries to an
// OpenTelemetry Histogram aggregation.
func convertHistogram(
	labelKeys []ocmetricdata.LabelKey,
	ts []*ocmetricdata.TimeSeries,
) (metricdata.Histogram[float64], error) {
	points := make([]metricdata.HistogramDataPoint[float64], 0, len(ts))
	var err error
	for _, t := range ts {
		attrs, attrsErr := convertAttrs(labelKeys, t.LabelValues)
		if attrsErr != nil {
			err = errors.Join(err, attrsErr)
			continue
		}
		for _, p := range t.Points {
			dist, ok := p.Value.(*ocmetricdata.Distribution)
			if !ok {
				err = errors.Join(err, fmt.Errorf("%w: %d", errMismatchedValueTypes, p.Value))
				continue
			}
			bucketCounts, exemplars, bucketErr := convertBuckets(dist.Buckets)
			if bucketErr != nil {
				err = errors.Join(err, bucketErr)
				continue
			}
			if dist.Count < 0 {
				err = errors.Join(err, fmt.Errorf("%w: %d", errNegativeCount, dist.Count))
				continue
			}
			points = append(points, metricdata.HistogramDataPoint[float64]{
				Attributes:   attrs,
				StartTime:    t.StartTime,
				Time:         p.Time,
				Count:        uint64(max(0, dist.Count)), // nolint:gosec // A count should never be negative.
				Sum:          dist.Sum,
				Bounds:       dist.BucketOptions.Bounds,
				BucketCounts: bucketCounts,
				Exemplars:    exemplars,
			})
		}
	}
	return metricdata.Histogram[float64]{DataPoints: points, Temporality: metricdata.CumulativeTemporality}, err
}

// convertBuckets converts from OpenCensus bucket counts to slice of uint64,
// and converts OpenCensus exemplars to OpenTelemetry exemplars.
func convertBuckets(buckets []ocmetricdata.Bucket) ([]uint64, []metricdata.Exemplar[float64], error) {
	bucketCounts := make([]uint64, len(buckets))
	exemplars := []metricdata.Exemplar[float64]{}
	var err error
	for i, bucket := range buckets {
		if bucket.Count < 0 {
			err = errors.Join(err, fmt.Errorf("%w: %q", errNegativeBucketCount, bucket.Count))
			continue
		}
		bucketCounts[i] = uint64(max(0, bucket.Count)) // nolint:gosec // A count should never be negative.

		if bucket.Exemplar != nil {
			exemplar, exemplarErr := convertExemplar(bucket.Exemplar)
			if exemplarErr != nil {
				err = errors.Join(err, exemplarErr)
				continue
			}
			exemplars = append(exemplars, exemplar)
		}
	}
	return bucketCounts, exemplars, err
}

// convertExemplar converts an OpenCensus exemplar to an OpenTelemetry exemplar.
func convertExemplar(ocExemplar *ocmetricdata.Exemplar) (metricdata.Exemplar[float64], error) {
	exemplar := metricdata.Exemplar[float64]{
		Value: ocExemplar.Value,
		Time:  ocExemplar.Timestamp,
	}
	var err error
	for k, v := range ocExemplar.Attachments {
		switch k {
		case ocmetricdata.AttachmentKeySpanContext:
			sc, ok := v.(octrace.SpanContext)
			if !ok {
				err = errors.Join(err, fmt.Errorf("%w; type: %v", errInvalidExemplarSpanContext, reflect.TypeOf(v)))
				continue
			}
			exemplar.SpanID = sc.SpanID[:]
			exemplar.TraceID = sc.TraceID[:]
		default:
			exemplar.FilteredAttributes = append(exemplar.FilteredAttributes, convertKV(k, v))
		}
	}
	slices.SortFunc(exemplar.FilteredAttributes, func(a, b attribute.KeyValue) int {
		return cmp.Compare(a.Key, b.Key)
	})
	return exemplar, err
}

// convertKV converts an OpenCensus Attachment to an OpenTelemetry KeyValue.
func convertKV(key string, value any) attribute.KeyValue {
	switch typedVal := value.(type) {
	case bool:
		return attribute.Bool(key, typedVal)
	case int:
		return attribute.Int(key, typedVal)
	case int8:
		return attribute.Int(key, int(typedVal))
	case int16:
		return attribute.Int(key, int(typedVal))
	case int32:
		return attribute.Int(key, int(typedVal))
	case int64:
		return attribute.Int64(key, typedVal)
	case uint:
		return uintKV(key, typedVal)
	case uint8:
		return uintKV(key, uint(typedVal))
	case uint16:
		return uintKV(key, uint(typedVal))
	case uint32:
		return uintKV(key, uint(typedVal))
	case uintptr:
		return uint64KV(key, uint64(typedVal))
	case uint64:
		return uint64KV(key, typedVal)
	case float32:
		return attribute.Float64(key, float64(typedVal))
	case float64:
		return attribute.Float64(key, typedVal)
	case complex64:
		return attribute.String(key, complexToString(typedVal))
	case complex128:
		return attribute.String(key, complexToString(typedVal))
	case string:
		return attribute.String(key, typedVal)
	case []bool:
		return attribute.BoolSlice(key, typedVal)
	case []int:
		return attribute.IntSlice(key, typedVal)
	case []int8:
		return intSliceKV(key, typedVal)
	case []int16:
		return intSliceKV(key, typedVal)
	case []int32:
		return intSliceKV(key, typedVal)
	case []int64:
		return attribute.Int64Slice(key, typedVal)
	case []uint:
		return uintSliceKV(key, typedVal)
	case []uint8:
		return uintSliceKV(key, typedVal)
	case []uint16:
		return uintSliceKV(key, typedVal)
	case []uint32:
		return uintSliceKV(key, typedVal)
	case []uintptr:
		return uintSliceKV(key, typedVal)
	case []uint64:
		return uintSliceKV(key, typedVal)
	case []float32:
		floatSlice := make([]float64, len(typedVal))
		for i := range typedVal {
			floatSlice[i] = float64(typedVal[i])
		}
		return attribute.Float64Slice(key, floatSlice)
	case []float64:
		return attribute.Float64Slice(key, typedVal)
	case []complex64:
		return complexSliceKV(key, typedVal)
	case []complex128:
		return complexSliceKV(key, typedVal)
	case []string:
		return attribute.StringSlice(key, typedVal)
	case fmt.Stringer:
		return attribute.Stringer(key, typedVal)
	default:
		return attribute.String(key, fmt.Sprintf("unhandled attribute value: %+v", value))
	}
}

func intSliceKV[N int8 | int16 | int32](key string, val []N) attribute.KeyValue {
	intSlice := make([]int, len(val))
	for i := range val {
		intSlice[i] = int(val[i])
	}
	return attribute.IntSlice(key, intSlice)
}

func uintKV(key string, val uint) attribute.KeyValue {
	if val > uint(math.MaxInt) {
		return attribute.String(key, strconv.FormatUint(uint64(val), 10))
	}
	return attribute.Int(key, int(val)) // nolint: gosec  // Overflow checked above.
}

func uintSliceKV[N uint | uint8 | uint16 | uint32 | uint64 | uintptr](key string, val []N) attribute.KeyValue {
	strSlice := make([]string, len(val))
	for i := range val {
		strSlice[i] = strconv.FormatUint(uint64(val[i]), 10)
	}
	return attribute.StringSlice(key, strSlice)
}

func uint64KV(key string, val uint64) attribute.KeyValue {
	const maxInt64 = ^uint64(0) >> 1
	if val > maxInt64 {
		return attribute.String(key, strconv.FormatUint(val, 10))
	}
	return attribute.Int64(key, int64(val)) // nolint: gosec  // Overflow checked above.
}

func complexSliceKV[N complex64 | complex128](key string, val []N) attribute.KeyValue {
	strSlice := make([]string, len(val))
	for i := range val {
		strSlice[i] = complexToString(val[i])
	}
	return attribute.StringSlice(key, strSlice)
}

func complexToString[N complex64 | complex128](val N) string {
	return strconv.FormatComplex(complex128(val), 'f', -1, 64)
}

// convertSummary converts OpenCensus Summary timeseries to an
// OpenTelemetry Summary.
func convertSummary(labelKeys []ocmetricdata.LabelKey, ts []*ocmetricdata.TimeSeries) (metricdata.Summary, error) {
	points := make([]metricdata.SummaryDataPoint, 0, len(ts))
	var err error
	for _, t := range ts {
		attrs, attrErr := convertAttrs(labelKeys, t.LabelValues)
		if attrErr != nil {
			err = errors.Join(err, attrErr)
			continue
		}
		for _, p := range t.Points {
			summary, ok := p.Value.(*ocmetricdata.Summary)
			if !ok {
				err = errors.Join(err, fmt.Errorf("%w: %d", errMismatchedValueTypes, p.Value))
				continue
			}
			if summary.Count < 0 {
				err = errors.Join(err, fmt.Errorf("%w: %d", errNegativeCount, summary.Count))
				continue
			}
			point := metricdata.SummaryDataPoint{
				Attributes:     attrs,
				StartTime:      t.StartTime,
				Time:           p.Time,
				Count:          uint64(max(0, summary.Count)), // nolint:gosec // A count should never be negative.
				QuantileValues: convertQuantiles(summary.Snapshot),
				Sum:            summary.Sum,
			}
			points = append(points, point)
		}
	}
	return metricdata.Summary{DataPoints: points}, err
}

// convertQuantiles converts an OpenCensus summary snapshot to
// OpenTelemetry quantiles.
func convertQuantiles(snapshot ocmetricdata.Snapshot) []metricdata.QuantileValue {
	quantileValues := make([]metricdata.QuantileValue, 0, len(snapshot.Percentiles))
	for quantile, value := range snapshot.Percentiles {
		quantileValues = append(quantileValues, metricdata.QuantileValue{
			// OpenCensus quantiles are range (0-100.0], but OpenTelemetry
			// quantiles are range [0.0, 1.0].
			Quantile: quantile / 100.0,
			Value:    value,
		})
	}
	slices.SortFunc(quantileValues, func(a, b metricdata.QuantileValue) int {
		return cmp.Compare(a.Quantile, b.Quantile)
	})
	return quantileValues
}

// convertAttrs converts from OpenCensus attribute keys and values to an
// OpenTelemetry attribute Set.
func convertAttrs(keys []ocmetricdata.LabelKey, values []ocmetricdata.LabelValue) (attribute.Set, error) {
	if len(keys) != len(values) {
		return attribute.NewSet(), fmt.Errorf(
			"%w: keys(%q) values(%q)",
			errMismatchedAttributeKeyValues,
			len(keys),
			len(values),
		)
	}
	attrs := []attribute.KeyValue{}
	for i, lv := range values {
		if !lv.Present {
			continue
		}
		attrs = append(attrs, attribute.KeyValue{
			Key:   attribute.Key(keys[i].Key),
			Value: attribute.StringValue(lv.Value),
		})
	}
	return attribute.NewSet(attrs...), nil
}
