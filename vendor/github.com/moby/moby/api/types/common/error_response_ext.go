package common

// Error returns the error message
func (e ErrorResponse) Error() string {
	return e.Message
}
