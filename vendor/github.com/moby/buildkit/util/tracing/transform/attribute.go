package transform

import (
	"go.opentelemetry.io/otel/attribute"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

// Attributes transforms a slice of OTLP attribute key-values into a slice of KeyValues
func Attributes(attrs []*commonpb.KeyValue) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}

	out := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		kv := attribute.KeyValue{
			Key:   attribute.Key(a.Key),
			Value: toValue(a.Value),
		}
		out = append(out, kv)
	}
	return out
}

func toValue(v *commonpb.AnyValue) attribute.Value {
	switch vv := v.Value.(type) {
	case *commonpb.AnyValue_BoolValue:
		return attribute.BoolValue(vv.BoolValue)
	case *commonpb.AnyValue_IntValue:
		return attribute.Int64Value(vv.IntValue)
	case *commonpb.AnyValue_DoubleValue:
		return attribute.Float64Value(vv.DoubleValue)
	case *commonpb.AnyValue_StringValue:
		return attribute.StringValue(vv.StringValue)
	case *commonpb.AnyValue_ArrayValue:
		return arrayValues(vv.ArrayValue.Values)
	default:
		return attribute.StringValue("INVALID")
	}
}

func boolArray(kv []*commonpb.AnyValue) attribute.Value {
	arr := make([]bool, len(kv))
	for i, v := range kv {
		arr[i] = v.GetBoolValue()
	}
	return attribute.BoolSliceValue(arr)
}

func intArray(kv []*commonpb.AnyValue) attribute.Value {
	arr := make([]int64, len(kv))
	for i, v := range kv {
		arr[i] = v.GetIntValue()
	}
	return attribute.Int64SliceValue(arr)
}

func doubleArray(kv []*commonpb.AnyValue) attribute.Value {
	arr := make([]float64, len(kv))
	for i, v := range kv {
		arr[i] = v.GetDoubleValue()
	}
	return attribute.Float64SliceValue(arr)
}

func stringArray(kv []*commonpb.AnyValue) attribute.Value {
	arr := make([]string, len(kv))
	for i, v := range kv {
		arr[i] = v.GetStringValue()
	}
	return attribute.StringSliceValue(arr)
}

func arrayValues(kv []*commonpb.AnyValue) attribute.Value {
	if len(kv) == 0 {
		return attribute.StringSliceValue([]string{})
	}

	switch kv[0].Value.(type) {
	case *commonpb.AnyValue_BoolValue:
		return boolArray(kv)
	case *commonpb.AnyValue_IntValue:
		return intArray(kv)
	case *commonpb.AnyValue_DoubleValue:
		return doubleArray(kv)
	case *commonpb.AnyValue_StringValue:
		return stringArray(kv)
	default:
		return attribute.StringSliceValue([]string{})
	}
}
