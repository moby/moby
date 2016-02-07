package match

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Contains struct {
	Needle string
	Not    bool
}

func (self Contains) Match(s string) bool {
	return strings.Contains(s, self.Needle) != self.Not
}

func (self Contains) Index(s string) (int, []int) {
	var (
		sub    string
		offset int
	)

	idx := strings.Index(s, self.Needle)

	if !self.Not {
		if idx == -1 {
			return -1, nil
		}

		offset = idx + len(self.Needle)

		if len(s) <= offset {
			return 0, []int{offset}
		}

		sub = s[offset:]
	} else {
		switch idx {
		case -1:
			sub = s
		default:
			sub = s[:idx]
		}
	}

	segments := make([]int, 0, utf8.RuneCountInString(sub)+1)
	for i, _ := range sub {
		segments = append(segments, offset+i)
	}

	return 0, append(segments, offset+len(sub))
}

func (self Contains) Len() int {
	return lenNo
}

func (self Contains) String() string {
	var not string
	if self.Not {
		not = "!"
	}
	return fmt.Sprintf("<contains:%s[%s]>", not, self.Needle)
}
