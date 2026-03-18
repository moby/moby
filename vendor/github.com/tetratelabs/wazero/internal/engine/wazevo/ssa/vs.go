package ssa

import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

// Variable is a unique identifier for a source program's variable and will correspond to
// multiple ssa Value(s).
//
// For example, `Local 1` is a Variable in WebAssembly, and Value(s) will be created for it
// whenever it executes `local.set 1`.
//
// Variable is useful to track the SSA Values of a variable in the source program, and
// can be used to find the corresponding latest SSA Value via Builder.FindValue.
//
// Higher 4-bit is used to store Type for this variable.
type Variable uint32

// String implements fmt.Stringer.
func (v Variable) String() string {
	return fmt.Sprintf("var%d", v&0x0fffffff)
}

func (v Variable) setType(typ Type) Variable {
	if v >= 1<<28 {
		panic(fmt.Sprintf("Too large variable: %d", v))
	}
	return Variable(typ)<<28 | v
}

func (v Variable) getType() Type {
	return Type(v >> 28)
}

// Value represents an SSA value with a type information. The relationship with Variable is 1: N (including 0),
// that means there might be multiple Variable(s) for a Value.
//
// 32 to 59-bit is used to store the unique identifier of the Instruction that generates this value if any.
// 60 to 63-bit is used to store Type for this value.
type Value uint64

// ValueID is the lower 32bit of Value, which is the pure identifier of Value without type info.
type ValueID uint32

const (
	valueIDInvalid ValueID = math.MaxUint32
	ValueInvalid           = Value(valueIDInvalid)
)

// Format creates a debug string for this Value using the data stored in Builder.
func (v Value) Format(b Builder) string {
	if annotation, ok := b.(*builder).valueAnnotations[v.ID()]; ok {
		return annotation
	}
	return fmt.Sprintf("v%d", v.ID())
}

func (v Value) formatWithType(b Builder) (ret string) {
	if annotation, ok := b.(*builder).valueAnnotations[v.ID()]; ok {
		ret = annotation + ":" + v.Type().String()
	} else {
		ret = fmt.Sprintf("v%d:%s", v.ID(), v.Type())
	}

	if wazevoapi.SSALoggingEnabled { // This is useful to check live value analysis bugs.
		if bd := b.(*builder); bd.donePostBlockLayoutPasses {
			id := v.ID()
			ret += fmt.Sprintf("(ref=%d)", bd.valuesInfo[id].RefCount)
		}
	}
	return ret
}

// Valid returns true if this value is valid.
func (v Value) Valid() bool {
	return v.ID() != valueIDInvalid
}

// Type returns the Type of this value.
func (v Value) Type() Type {
	return Type(v >> 60)
}

// ID returns the valueID of this value.
func (v Value) ID() ValueID {
	return ValueID(v)
}

// setType sets a type to this Value and returns the updated Value.
func (v Value) setType(typ Type) Value {
	return v | Value(typ)<<60
}

// setInstructionID sets an Instruction.id to this Value and returns the updated Value.
func (v Value) setInstructionID(id int) Value {
	if id < 0 || uint(id) >= 1<<28 {
		panic(fmt.Sprintf("Too large instruction ID: %d", id))
	}
	return v | Value(id)<<32
}

// instructionID() returns the Instruction.id of this Value.
func (v Value) instructionID() int {
	return int(v>>32) & 0x0fffffff
}

// Values is a slice of Value. Use this instead of []Value to reuse the underlying memory.
type Values = wazevoapi.VarLength[Value]

// ValuesNil is a nil Values.
var ValuesNil = wazevoapi.NewNilVarLength[Value]()
