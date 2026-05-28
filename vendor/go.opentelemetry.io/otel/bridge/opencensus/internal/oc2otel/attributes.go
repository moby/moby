// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package oc2otel provides conversion from OpenCensus to OpenTelemetry.
package oc2otel // import "go.opentelemetry.io/otel/bridge/opencensus/internal/oc2otel"

import (
	octrace "go.opencensus.io/trace"

	"go.opentelemetry.io/otel/attribute"
)

func Attributes(attr []octrace.Attribute) []attribute.KeyValue {
	otelAttr := make([]attribute.KeyValue, len(attr))
	for i, a := range attr {
		otelAttr[i] = attribute.KeyValue{
			Key:   attribute.Key(a.Key()),
			Value: AttributeValue(a.Value()),
		}
	}
	return otelAttr
}

func AttributesFromMap(attr map[string]any) []attribute.KeyValue {
	otelAttr := make([]attribute.KeyValue, 0, len(attr))
	for k, v := range attr {
		otelAttr = append(otelAttr, attribute.KeyValue{
			Key:   attribute.Key(k),
			Value: AttributeValue(v),
		})
	}
	return otelAttr
}

func AttributeValue(ocval any) attribute.Value {
	switch v := ocval.(type) {
	case bool:
		return attribute.BoolValue(v)
	case int64:
		return attribute.Int64Value(v)
	case float64:
		return attribute.Float64Value(v)
	case string:
		return attribute.StringValue(v)
	default:
		return attribute.StringValue("unknown")
	}
}
