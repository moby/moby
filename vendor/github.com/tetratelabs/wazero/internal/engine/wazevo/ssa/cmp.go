package ssa

// IntegerCmpCond represents a condition for integer comparison.
type IntegerCmpCond byte

const (
	// IntegerCmpCondInvalid represents an invalid condition.
	IntegerCmpCondInvalid IntegerCmpCond = iota
	// IntegerCmpCondEqual represents "==".
	IntegerCmpCondEqual
	// IntegerCmpCondNotEqual represents "!=".
	IntegerCmpCondNotEqual
	// IntegerCmpCondSignedLessThan represents Signed "<".
	IntegerCmpCondSignedLessThan
	// IntegerCmpCondSignedGreaterThanOrEqual represents Signed ">=".
	IntegerCmpCondSignedGreaterThanOrEqual
	// IntegerCmpCondSignedGreaterThan represents Signed ">".
	IntegerCmpCondSignedGreaterThan
	// IntegerCmpCondSignedLessThanOrEqual represents Signed "<=".
	IntegerCmpCondSignedLessThanOrEqual
	// IntegerCmpCondUnsignedLessThan represents Unsigned "<".
	IntegerCmpCondUnsignedLessThan
	// IntegerCmpCondUnsignedGreaterThanOrEqual represents Unsigned ">=".
	IntegerCmpCondUnsignedGreaterThanOrEqual
	// IntegerCmpCondUnsignedGreaterThan represents Unsigned ">".
	IntegerCmpCondUnsignedGreaterThan
	// IntegerCmpCondUnsignedLessThanOrEqual represents Unsigned "<=".
	IntegerCmpCondUnsignedLessThanOrEqual
)

// String implements fmt.Stringer.
func (i IntegerCmpCond) String() string {
	switch i {
	case IntegerCmpCondEqual:
		return "eq"
	case IntegerCmpCondNotEqual:
		return "neq"
	case IntegerCmpCondSignedLessThan:
		return "lt_s"
	case IntegerCmpCondSignedGreaterThanOrEqual:
		return "ge_s"
	case IntegerCmpCondSignedGreaterThan:
		return "gt_s"
	case IntegerCmpCondSignedLessThanOrEqual:
		return "le_s"
	case IntegerCmpCondUnsignedLessThan:
		return "lt_u"
	case IntegerCmpCondUnsignedGreaterThanOrEqual:
		return "ge_u"
	case IntegerCmpCondUnsignedGreaterThan:
		return "gt_u"
	case IntegerCmpCondUnsignedLessThanOrEqual:
		return "le_u"
	default:
		panic("invalid integer comparison condition")
	}
}

// Signed returns true if the condition is signed integer comparison.
func (i IntegerCmpCond) Signed() bool {
	switch i {
	case IntegerCmpCondSignedLessThan, IntegerCmpCondSignedGreaterThanOrEqual,
		IntegerCmpCondSignedGreaterThan, IntegerCmpCondSignedLessThanOrEqual:
		return true
	default:
		return false
	}
}

type FloatCmpCond byte

const (
	// FloatCmpCondInvalid represents an invalid condition.
	FloatCmpCondInvalid FloatCmpCond = iota
	// FloatCmpCondEqual represents "==".
	FloatCmpCondEqual
	// FloatCmpCondNotEqual represents "!=".
	FloatCmpCondNotEqual
	// FloatCmpCondLessThan represents "<".
	FloatCmpCondLessThan
	// FloatCmpCondLessThanOrEqual represents "<=".
	FloatCmpCondLessThanOrEqual
	// FloatCmpCondGreaterThan represents ">".
	FloatCmpCondGreaterThan
	// FloatCmpCondGreaterThanOrEqual represents ">=".
	FloatCmpCondGreaterThanOrEqual
)

// String implements fmt.Stringer.
func (f FloatCmpCond) String() string {
	switch f {
	case FloatCmpCondEqual:
		return "eq"
	case FloatCmpCondNotEqual:
		return "neq"
	case FloatCmpCondLessThan:
		return "lt"
	case FloatCmpCondLessThanOrEqual:
		return "le"
	case FloatCmpCondGreaterThan:
		return "gt"
	case FloatCmpCondGreaterThanOrEqual:
		return "ge"
	default:
		panic("invalid float comparison condition")
	}
}
