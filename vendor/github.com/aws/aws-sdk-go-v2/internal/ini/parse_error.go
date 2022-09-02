package ini

// ParseError is an error which is returned during any part of
// the parsing process.
type ParseError struct {
	msg string
}

// NewParseError will return a new ParseError where message
// is the description of the error.
func NewParseError(message string) *ParseError {
	return &ParseError{
		msg: message,
	}
}

func (err *ParseError) Error() string {
	return err.msg
}
