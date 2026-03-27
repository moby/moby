package unstable

import "fmt"

// Kind represents the type of TOML structure contained in a given Node.
type Kind int

const (
	// Invalid represents an invalid meta node.
	Invalid Kind = iota
	// Comment represents a comment meta node.
	Comment
	// Key represents a key meta node.
	Key

	// Table represents a top-level table.
	Table
	// ArrayTable represents a top-level array table.
	ArrayTable
	// KeyValue represents a top-level key value.
	KeyValue

	// Array represents an array container value.
	Array
	// InlineTable represents an inline table container value.
	InlineTable

	// String represents a string value.
	String
	// Bool represents a boolean value.
	Bool
	// Float represents a floating point value.
	Float
	// Integer represents an integer value.
	Integer
	// LocalDate represents a a local date value.
	LocalDate
	// LocalTime represents a local time value.
	LocalTime
	// LocalDateTime represents a local date/time value.
	LocalDateTime
	// DateTime represents a data/time value.
	DateTime
)

// String implementation of fmt.Stringer.
func (k Kind) String() string {
	switch k {
	case Invalid:
		return "Invalid"
	case Comment:
		return "Comment"
	case Key:
		return "Key"
	case Table:
		return "Table"
	case ArrayTable:
		return "ArrayTable"
	case KeyValue:
		return "KeyValue"
	case Array:
		return "Array"
	case InlineTable:
		return "InlineTable"
	case String:
		return "String"
	case Bool:
		return "Bool"
	case Float:
		return "Float"
	case Integer:
		return "Integer"
	case LocalDate:
		return "LocalDate"
	case LocalTime:
		return "LocalTime"
	case LocalDateTime:
		return "LocalDateTime"
	case DateTime:
		return "DateTime"
	}
	panic(fmt.Errorf("Kind.String() not implemented for '%d'", k))
}
