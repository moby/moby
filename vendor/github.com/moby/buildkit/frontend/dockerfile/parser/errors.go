package parser

import (
	"github.com/moby/buildkit/util/stack"
	"github.com/pkg/errors"
)

// ErrorLocation gives a location in source code that caused the error
type ErrorLocation struct {
	Locations [][]Range
	error
}

// Unwrap unwraps to the next error
func (e *ErrorLocation) Unwrap() error {
	return e.error
}

// Range is a code section between two positions
type Range struct {
	Start Position
	End   Position
}

// Position is a point in source code
type Position struct {
	Line      int
	Character int
}

func withLocation(err error, start, end int) error {
	return WithLocation(err, toRanges(start, end))
}

// WithLocation extends an error with a source code location
func WithLocation(err error, location []Range) error {
	return setLocation(err, location, true)
}

func SetLocation(err error, location []Range) error {
	return setLocation(err, location, false)
}

func setLocation(err error, location []Range, add bool) error {
	if err == nil {
		return nil
	}
	var el *ErrorLocation
	if errors.As(err, &el) {
		if add {
			el.Locations = append(el.Locations, location)
		} else {
			el.Locations = [][]Range{location}
		}
		return err
	}
	return stack.Enable(&ErrorLocation{
		error:     err,
		Locations: [][]Range{location},
	})
}

func toRanges(start, end int) (r []Range) {
	if end <= start {
		end = start
	}
	for i := start; i <= end; i++ {
		r = append(r, Range{Start: Position{Line: i}, End: Position{Line: i}})
	}
	return
}
