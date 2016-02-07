package match

import (
	"fmt"
	"unicode/utf8"
)

type Min struct {
	Limit int
}

func (self Min) Match(s string) bool {
	var l int
	for range s {
		l += 1
		if l >= self.Limit {
			return true
		}
	}

	return false
}

func (self Min) Index(s string) (int, []int) {
	var count int

	c := utf8.RuneCountInString(s)
	if c < self.Limit {
		return -1, nil
	}

	segments := make([]int, 0, c-self.Limit+1)
	for i, r := range s {
		count++
		if count >= self.Limit {
			segments = append(segments, i+utf8.RuneLen(r))
		}
	}

	return 0, segments
}

func (self Min) Len() int {
	return lenNo
}

func (self Min) String() string {
	return fmt.Sprintf("<min:%d>", self.Limit)
}
