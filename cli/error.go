package cli

import "strings"

// Errors is a list of errors.
// Useful in a loop if you don't want to return the error right away and you want to display after the loop,
// all the errors that happened during the loop.
type Errors []error

func (errList Errors) Error() string {
	if len(errList) < 1 {
		return ""
	}

	out := make([]string, len(errList))
	for i := range errList {
		out[i] = errList[i].Error()
	}
	return strings.Join(out, ", ")
}
