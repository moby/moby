package match

import (
	"fmt"
	"unicode/utf8"
)

type Max struct {
	Limit int
}

func (self Max) Match(s string) bool {
	var l int
	for range s {
		l += 1
		if l > self.Limit {
			return false
		}
	}

	return true
}

func (self Max) Index(s string) (index int, segments []int) {
	segments = append(segments, 0)
	var count int
	for i, r := range s {
		count++
		if count > self.Limit {
			break
		}
		segments = append(segments, i+utf8.RuneLen(r))
	}

	return 0, segments
}

func (self Max) Len() int {
	return lenNo
}

func (self Max) String() string {
	return fmt.Sprintf("<max:%d>", self.Limit)
}
