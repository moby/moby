package etw

import (
	"math"
	"unsafe"
)

// FieldOpt defines the option function type that can be passed to
// Provider.WriteEvent to add fields to the event.
type FieldOpt func(em *EventMetadata, ed *EventData)

// WithFields returns the variadic arguments as a single slice.
func WithFields(opts ...FieldOpt) []FieldOpt {
	return opts
}

// BoolField adds a single bool field to the event.
func BoolField(name string, value bool) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeUint8, OutTypeBoolean, 0)
		bool8 := uint8(0)
		if value {
			bool8 = uint8(1)
		}
		ed.WriteUint8(bool8)
	}
}

// BoolArray adds an array of bool to the event.
func BoolArray(name string, values []bool) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeUint8, OutTypeBoolean, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			bool8 := uint8(0)
			if v {
				bool8 = uint8(1)
			}
			ed.WriteUint8(bool8)
		}
	}
}

// StringField adds a single string field to the event.
func StringField(name string, value string) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeANSIString, OutTypeUTF8, 0)
		ed.WriteString(value)
	}
}

// StringArray adds an array of string to the event.
func StringArray(name string, values []string) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeANSIString, OutTypeUTF8, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteString(v)
		}
	}
}

// IntField adds a single int field to the event.
func IntField(name string, value int) FieldOpt {
	switch unsafe.Sizeof(value) {
	case 4:
		return Int32Field(name, int32(value))
	case 8:
		return Int64Field(name, int64(value))
	default:
		panic("Unsupported int size")
	}
}

// IntArray adds an array of int to the event.
func IntArray(name string, values []int) FieldOpt {
	inType := InTypeNull
	var writeItem func(*EventData, int)
	switch unsafe.Sizeof(values[0]) {
	case 4:
		inType = InTypeInt32
		writeItem = func(ed *EventData, item int) { ed.WriteInt32(int32(item)) }
	case 8:
		inType = InTypeInt64
		writeItem = func(ed *EventData, item int) { ed.WriteInt64(int64(item)) }
	default:
		panic("Unsupported int size")
	}

	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, inType, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			writeItem(ed, v)
		}
	}
}

// Int8Field adds a single int8 field to the event.
func Int8Field(name string, value int8) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeInt8, OutTypeDefault, 0)
		ed.WriteInt8(value)
	}
}

// Int8Array adds an array of int8 to the event.
func Int8Array(name string, values []int8) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeInt8, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteInt8(v)
		}
	}
}

// Int16Field adds a single int16 field to the event.
func Int16Field(name string, value int16) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeInt16, OutTypeDefault, 0)
		ed.WriteInt16(value)
	}
}

// Int16Array adds an array of int16 to the event.
func Int16Array(name string, values []int16) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeInt16, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteInt16(v)
		}
	}
}

// Int32Field adds a single int32 field to the event.
func Int32Field(name string, value int32) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeInt32, OutTypeDefault, 0)
		ed.WriteInt32(value)
	}
}

// Int32Array adds an array of int32 to the event.
func Int32Array(name string, values []int32) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeInt32, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteInt32(v)
		}
	}
}

// Int64Field adds a single int64 field to the event.
func Int64Field(name string, value int64) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeInt64, OutTypeDefault, 0)
		ed.WriteInt64(value)
	}
}

// Int64Array adds an array of int64 to the event.
func Int64Array(name string, values []int64) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeInt64, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteInt64(v)
		}
	}
}

// UintField adds a single uint field to the event.
func UintField(name string, value uint) FieldOpt {
	switch unsafe.Sizeof(value) {
	case 4:
		return Uint32Field(name, uint32(value))
	case 8:
		return Uint64Field(name, uint64(value))
	default:
		panic("Unsupported uint size")
	}
}

// UintArray adds an array of uint to the event.
func UintArray(name string, values []uint) FieldOpt {
	inType := InTypeNull
	var writeItem func(*EventData, uint)
	switch unsafe.Sizeof(values[0]) {
	case 4:
		inType = InTypeUint32
		writeItem = func(ed *EventData, item uint) { ed.WriteUint32(uint32(item)) }
	case 8:
		inType = InTypeUint64
		writeItem = func(ed *EventData, item uint) { ed.WriteUint64(uint64(item)) }
	default:
		panic("Unsupported uint size")
	}

	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, inType, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			writeItem(ed, v)
		}
	}
}

// Uint8Field adds a single uint8 field to the event.
func Uint8Field(name string, value uint8) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeUint8, OutTypeDefault, 0)
		ed.WriteUint8(value)
	}
}

// Uint8Array adds an array of uint8 to the event.
func Uint8Array(name string, values []uint8) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeUint8, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteUint8(v)
		}
	}
}

// Uint16Field adds a single uint16 field to the event.
func Uint16Field(name string, value uint16) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeUint16, OutTypeDefault, 0)
		ed.WriteUint16(value)
	}
}

// Uint16Array adds an array of uint16 to the event.
func Uint16Array(name string, values []uint16) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeUint16, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteUint16(v)
		}
	}
}

// Uint32Field adds a single uint32 field to the event.
func Uint32Field(name string, value uint32) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeUint32, OutTypeDefault, 0)
		ed.WriteUint32(value)
	}
}

// Uint32Array adds an array of uint32 to the event.
func Uint32Array(name string, values []uint32) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeUint32, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteUint32(v)
		}
	}
}

// Uint64Field adds a single uint64 field to the event.
func Uint64Field(name string, value uint64) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeUint64, OutTypeDefault, 0)
		ed.WriteUint64(value)
	}
}

// Uint64Array adds an array of uint64 to the event.
func Uint64Array(name string, values []uint64) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeUint64, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteUint64(v)
		}
	}
}

// UintptrField adds a single uintptr field to the event.
func UintptrField(name string, value uintptr) FieldOpt {
	inType := InTypeNull
	var writeItem func(*EventData, uintptr)
	switch unsafe.Sizeof(value) {
	case 4:
		inType = InTypeHexInt32
		writeItem = func(ed *EventData, item uintptr) { ed.WriteUint32(uint32(item)) }
	case 8:
		inType = InTypeHexInt64
		writeItem = func(ed *EventData, item uintptr) { ed.WriteUint64(uint64(item)) }
	default:
		panic("Unsupported uintptr size")
	}

	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, inType, OutTypeDefault, 0)
		writeItem(ed, value)
	}
}

// UintptrArray adds an array of uintptr to the event.
func UintptrArray(name string, values []uintptr) FieldOpt {
	inType := InTypeNull
	var writeItem func(*EventData, uintptr)
	switch unsafe.Sizeof(values[0]) {
	case 4:
		inType = InTypeHexInt32
		writeItem = func(ed *EventData, item uintptr) { ed.WriteUint32(uint32(item)) }
	case 8:
		inType = InTypeHexInt64
		writeItem = func(ed *EventData, item uintptr) { ed.WriteUint64(uint64(item)) }
	default:
		panic("Unsupported uintptr size")
	}

	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, inType, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			writeItem(ed, v)
		}
	}
}

// Float32Field adds a single float32 field to the event.
func Float32Field(name string, value float32) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeFloat, OutTypeDefault, 0)
		ed.WriteUint32(math.Float32bits(value))
	}
}

// Float32Array adds an array of float32 to the event.
func Float32Array(name string, values []float32) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeFloat, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteUint32(math.Float32bits(v))
		}
	}
}

// Float64Field adds a single float64 field to the event.
func Float64Field(name string, value float64) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteField(name, InTypeDouble, OutTypeDefault, 0)
		ed.WriteUint64(math.Float64bits(value))
	}
}

// Float64Array adds an array of float64 to the event.
func Float64Array(name string, values []float64) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteArray(name, InTypeDouble, OutTypeDefault, 0)
		ed.WriteUint16(uint16(len(values)))
		for _, v := range values {
			ed.WriteUint64(math.Float64bits(v))
		}
	}
}

// Struct adds a nested struct to the event, the FieldOpts in the opts argument
// are used to specify the fields of the struct.
func Struct(name string, opts ...FieldOpt) FieldOpt {
	return func(em *EventMetadata, ed *EventData) {
		em.WriteStruct(name, uint8(len(opts)), 0)
		for _, opt := range opts {
			opt(em, ed)
		}
	}
}
