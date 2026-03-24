package jsonstream

// Error wraps a concrete Code and Message, Code is
// an integer error code, Message is the error message.
type Error struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Message
}
