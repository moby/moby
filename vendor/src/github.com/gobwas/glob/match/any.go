package match

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Any struct {
	Separators string
}

func (self Any) Match(s string) bool {
	return strings.IndexAny(s, self.Separators) == -1
}

func (self Any) Index(s string) (int, []int) {
	var sub string

	found := strings.IndexAny(s, self.Separators)
	switch found {
	case -1:
		sub = s
	case 0:
		return 0, []int{0}
	default:
		sub = s[:found]
	}

	segments := make([]int, 0, utf8.RuneCountInString(sub)+1)
	for i := range sub {
		segments = append(segments, i)
	}

	segments = append(segments, len(sub))

	return 0, segments
}

func (self Any) Len() int {
	return lenNo
}

func (self Any) String() string {
	return fmt.Sprintf("<any:![%s]>", self.Separators)
}
