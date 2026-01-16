package jsonpointer

type pointerError string

func (e pointerError) Error() string {
	return string(e)
}

const (
	// ErrPointer is an error raised by the jsonpointer package
	ErrPointer pointerError = "JSON pointer error"

	// ErrInvalidStart states that a JSON pointer must start with a separator ("/")
	ErrInvalidStart pointerError = `JSON pointer must be empty or start with a "` + pointerSeparator

	// ErrUnsupportedValueType indicates that a value of the wrong type is being set
	ErrUnsupportedValueType pointerError = "only structs, pointers, maps and slices are supported for setting values"
)
