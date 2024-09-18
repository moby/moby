// Code created by gotmpl. DO NOT MODIFY.
// source: internal/shared/otlp/otlpmetric/transform/attribute.go.tmpl

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

package transform // import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/transform"

import (
	"go.opentelemetry.io/otel/attribute"
	cpb "go.opentelemetry.io/proto/otlp/common/v1"
)

// AttrIter transforms an attribute iterator into OTLP key-values.
func AttrIter(iter attribute.Iterator) []*cpb.KeyValue {
	l := iter.Len()
	if l == 0 {
		return nil
	}

	out := make([]*cpb.KeyValue, 0, l)
	for iter.Next() {
		out = append(out, KeyValue(iter.Attribute()))
	}
	return out
}

// KeyValues transforms a slice of attribute KeyValues into OTLP key-values.
func KeyValues(attrs []attribute.KeyValue) []*cpb.KeyValue {
	if len(attrs) == 0 {
		return nil
	}

	out := make([]*cpb.KeyValue, 0, len(attrs))
	for _, kv := range attrs {
		out = append(out, KeyValue(kv))
	}
	return out
}

// KeyValue transforms an attribute KeyValue into an OTLP key-value.
func KeyValue(kv attribute.KeyValue) *cpb.KeyValue {
	return &cpb.KeyValue{Key: string(kv.Key), Value: Value(kv.Value)}
}

// Value transforms an attribute Value into an OTLP AnyValue.
func Value(v attribute.Value) *cpb.AnyValue {
	av := new(cpb.AnyValue)
	switch v.Type() {
	case attribute.BOOL:
		av.Value = &cpb.AnyValue_BoolValue{
			BoolValue: v.AsBool(),
		}
	case attribute.BOOLSLICE:
		av.Value = &cpb.AnyValue_ArrayValue{
			ArrayValue: &cpb.ArrayValue{
				Values: boolSliceValues(v.AsBoolSlice()),
			},
		}
	case attribute.INT64:
		av.Value = &cpb.AnyValue_IntValue{
			IntValue: v.AsInt64(),
		}
	case attribute.INT64SLICE:
		av.Value = &cpb.AnyValue_ArrayValue{
			ArrayValue: &cpb.ArrayValue{
				Values: int64SliceValues(v.AsInt64Slice()),
			},
		}
	case attribute.FLOAT64:
		av.Value = &cpb.AnyValue_DoubleValue{
			DoubleValue: v.AsFloat64(),
		}
	case attribute.FLOAT64SLICE:
		av.Value = &cpb.AnyValue_ArrayValue{
			ArrayValue: &cpb.ArrayValue{
				Values: float64SliceValues(v.AsFloat64Slice()),
			},
		}
	case attribute.STRING:
		av.Value = &cpb.AnyValue_StringValue{
			StringValue: v.AsString(),
		}
	case attribute.STRINGSLICE:
		av.Value = &cpb.AnyValue_ArrayValue{
			ArrayValue: &cpb.ArrayValue{
				Values: stringSliceValues(v.AsStringSlice()),
			},
		}
	default:
		av.Value = &cpb.AnyValue_StringValue{
			StringValue: "INVALID",
		}
	}
	return av
}

func boolSliceValues(vals []bool) []*cpb.AnyValue {
	converted := make([]*cpb.AnyValue, len(vals))
	for i, v := range vals {
		converted[i] = &cpb.AnyValue{
			Value: &cpb.AnyValue_BoolValue{
				BoolValue: v,
			},
		}
	}
	return converted
}

func int64SliceValues(vals []int64) []*cpb.AnyValue {
	converted := make([]*cpb.AnyValue, len(vals))
	for i, v := range vals {
		converted[i] = &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{
				IntValue: v,
			},
		}
	}
	return converted
}

func float64SliceValues(vals []float64) []*cpb.AnyValue {
	converted := make([]*cpb.AnyValue, len(vals))
	for i, v := range vals {
		converted[i] = &cpb.AnyValue{
			Value: &cpb.AnyValue_DoubleValue{
				DoubleValue: v,
			},
		}
	}
	return converted
}

func stringSliceValues(vals []string) []*cpb.AnyValue {
	converted := make([]*cpb.AnyValue, len(vals))
	for i, v := range vals {
		converted[i] = &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{
				StringValue: v,
			},
		}
	}
	return converted
}
