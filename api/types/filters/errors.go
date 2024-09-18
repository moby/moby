package filters

import "fmt"

// invalidFilter indicates that the provided filter or its value is invalid
type invalidFilter struct {
	Filter string
	Value  []string
}

func (e invalidFilter) Error() string {
	msg := "invalid filter"
	if e.Filter != "" {
		msg += " '" + e.Filter
		if e.Value != nil {
			msg = fmt.Sprintf("%s=%s", msg, e.Value)
		}
		msg += "'"
	}
	return msg
}

// InvalidParameter marks this error as ErrInvalidParameter
func (e invalidFilter) InvalidParameter() {}

// unreachableCode is an error indicating that the code path was not expected to be reached.
type unreachableCode struct {
	Filter string
	Value  []string
}

// System marks this error as ErrSystem
func (e unreachableCode) System() {}

func (e unreachableCode) Error() string {
	return fmt.Sprintf("unreachable code reached for filter: %q with values: %s", e.Filter, e.Value)
}
