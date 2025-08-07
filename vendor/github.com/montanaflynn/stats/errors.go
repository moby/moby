package stats

type statsError struct {
	err string
}

func (s statsError) Error() string {
	return s.err
}

func (s statsError) String() string {
	return s.err
}

// These are the package-wide error values.
// All error identification should use these values.
// https://github.com/golang/go/wiki/Errors#naming
var (
	// ErrEmptyInput Input must not be empty
	ErrEmptyInput = statsError{"Input must not be empty."}
	// ErrNaN Not a number
	ErrNaN = statsError{"Not a number."}
	// ErrNegative Must not contain negative values
	ErrNegative = statsError{"Must not contain negative values."}
	// ErrZero Must not contain zero values
	ErrZero = statsError{"Must not contain zero values."}
	// ErrBounds Input is outside of range
	ErrBounds = statsError{"Input is outside of range."}
	// ErrSize Must be the same length
	ErrSize = statsError{"Must be the same length."}
	// ErrInfValue Value is infinite
	ErrInfValue = statsError{"Value is infinite."}
	// ErrYCoord Y Value must be greater than zero
	ErrYCoord = statsError{"Y Value must be greater than zero."}
)
