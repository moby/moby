package etwlogrus

import (
	"fmt"
	"reflect"

	"github.com/Microsoft/go-winio/internal/etw"
	"github.com/sirupsen/logrus"
)

// Hook is a Logrus hook which logs received events to ETW.
type Hook struct {
	provider *etw.Provider
}

// NewHook registers a new ETW provider and returns a hook to log from it.
func NewHook(providerName string) (*Hook, error) {
	hook := Hook{}

	provider, err := etw.NewProvider(providerName, nil)
	if err != nil {
		return nil, err
	}
	hook.provider = provider

	return &hook, nil
}

// Levels returns the set of levels that this hook wants to receive log entries
// for.
func (h *Hook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.TraceLevel,
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

// Fire receives each Logrus entry as it is logged, and logs it to ETW.
func (h *Hook) Fire(e *logrus.Entry) error {
	level := etw.Level(e.Level)
	if !h.provider.IsEnabledForLevel(level) {
		return nil
	}

	// Reserve extra space for the message field.
	fields := make([]etw.FieldOpt, 0, len(e.Data)+1)

	fields = append(fields, etw.StringField("Message", e.Message))

	for k, v := range e.Data {
		fields = append(fields, getFieldOpt(k, v))
	}

	// We could try to map Logrus levels to ETW levels, but we would lose some
	// fidelity as there are fewer ETW levels. So instead we use the level
	// directly.
	return h.provider.WriteEvent(
		"LogrusEntry",
		etw.WithEventOpts(etw.WithLevel(level)),
		fields)
}

// Currently, we support logging basic builtin types (int, string, etc), slices
// of basic builtin types, error, types derived from the basic types (e.g. "type
// foo int"), and structs (recursively logging their fields). We do not support
// slices of derived types (e.g. "[]foo").
//
// For types that we don't support, the value is formatted via fmt.Sprint, and
// we also log a message that the type is unsupported along with the formatted
// type. The intent of this is to make it easier to see which types are not
// supported in traces, so we can evaluate adding support for more types in the
// future.
func getFieldOpt(k string, v interface{}) etw.FieldOpt {
	switch v := v.(type) {
	case bool:
		return etw.BoolField(k, v)
	case []bool:
		return etw.BoolArray(k, v)
	case string:
		return etw.StringField(k, v)
	case []string:
		return etw.StringArray(k, v)
	case int:
		return etw.IntField(k, v)
	case []int:
		return etw.IntArray(k, v)
	case int8:
		return etw.Int8Field(k, v)
	case []int8:
		return etw.Int8Array(k, v)
	case int16:
		return etw.Int16Field(k, v)
	case []int16:
		return etw.Int16Array(k, v)
	case int32:
		return etw.Int32Field(k, v)
	case []int32:
		return etw.Int32Array(k, v)
	case int64:
		return etw.Int64Field(k, v)
	case []int64:
		return etw.Int64Array(k, v)
	case uint:
		return etw.UintField(k, v)
	case []uint:
		return etw.UintArray(k, v)
	case uint8:
		return etw.Uint8Field(k, v)
	case []uint8:
		return etw.Uint8Array(k, v)
	case uint16:
		return etw.Uint16Field(k, v)
	case []uint16:
		return etw.Uint16Array(k, v)
	case uint32:
		return etw.Uint32Field(k, v)
	case []uint32:
		return etw.Uint32Array(k, v)
	case uint64:
		return etw.Uint64Field(k, v)
	case []uint64:
		return etw.Uint64Array(k, v)
	case uintptr:
		return etw.UintptrField(k, v)
	case []uintptr:
		return etw.UintptrArray(k, v)
	case float32:
		return etw.Float32Field(k, v)
	case []float32:
		return etw.Float32Array(k, v)
	case float64:
		return etw.Float64Field(k, v)
	case []float64:
		return etw.Float64Array(k, v)
	case error:
		return etw.StringField(k, v.Error())
	default:
		switch rv := reflect.ValueOf(v); rv.Kind() {
		case reflect.Bool:
			return getFieldOpt(k, rv.Bool())
		case reflect.Int:
			return getFieldOpt(k, int(rv.Int()))
		case reflect.Int8:
			return getFieldOpt(k, int8(rv.Int()))
		case reflect.Int16:
			return getFieldOpt(k, int16(rv.Int()))
		case reflect.Int32:
			return getFieldOpt(k, int32(rv.Int()))
		case reflect.Int64:
			return getFieldOpt(k, int64(rv.Int()))
		case reflect.Uint:
			return getFieldOpt(k, uint(rv.Uint()))
		case reflect.Uint8:
			return getFieldOpt(k, uint8(rv.Uint()))
		case reflect.Uint16:
			return getFieldOpt(k, uint16(rv.Uint()))
		case reflect.Uint32:
			return getFieldOpt(k, uint32(rv.Uint()))
		case reflect.Uint64:
			return getFieldOpt(k, uint64(rv.Uint()))
		case reflect.Uintptr:
			return getFieldOpt(k, uintptr(rv.Uint()))
		case reflect.Float32:
			return getFieldOpt(k, float32(rv.Float()))
		case reflect.Float64:
			return getFieldOpt(k, float64(rv.Float()))
		case reflect.String:
			return getFieldOpt(k, rv.String())
		case reflect.Struct:
			fields := make([]etw.FieldOpt, 0, rv.NumField())
			for i := 0; i < rv.NumField(); i++ {
				field := rv.Field(i)
				if field.CanInterface() {
					fields = append(fields, getFieldOpt(k, field.Interface()))
				}
			}
			return etw.Struct(k, fields...)
		}
	}

	return etw.StringField(k, fmt.Sprintf("(Unsupported: %T) %v", v, v))
}

// Close cleans up the hook and closes the ETW provider.
func (h *Hook) Close() error {
	return h.provider.Close()
}
