package match

import (
	"fmt"
	"unicode/utf8"
)

type Super struct{}

func (self Super) Match(s string) bool {
	return true
}

func (self Super) Len() int {
	return lenNo
}

func (self Super) Index(s string) (int, []int) {
	segments := make([]int, 0, utf8.RuneCountInString(s)+1)
	for i := range s {
		segments = append(segments, i)
	}

	segments = append(segments, len(s))

	return 0, segments
}

func (self Super) String() string {
	return fmt.Sprintf("<super>")
}
