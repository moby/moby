package search

import "fmt"

type invalidFilter struct {
	filter string
	value  interface{}
}

func (e invalidFilter) Error() string {
	msg := "invalid filter '" + e.filter
	if e.value != nil {
		msg += fmt.Sprintf("=%s", e.value)
	}
	return msg + "'"
}

func (e invalidFilter) InvalidParameter() {}
