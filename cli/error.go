package cli

import "bytes"

// Errors is a list of errors.
// Useful in a loop if you don't want to return the error right away and you want to display after the loop,
// all the errors that happened during the loop.
type Errors []error

func (errs Errors) Error() string {
	if len(errs) < 1 {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString(errs[0].Error())
	for _, err := range errs[1:] {
		buf.WriteString(", ")
		buf.WriteString(err.Error())
	}
	return buf.String()
}
