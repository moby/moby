package match

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// single represents ?
type Single struct {
	Separators string
}

func (self Single) Match(s string) bool {
	r, w := utf8.DecodeRuneInString(s)
	if len(s) > w {
		return false
	}

	return strings.IndexRune(self.Separators, r) == -1
}

func (self Single) Len() int {
	return lenOne
}

func (self Single) Index(s string) (int, []int) {
	for i, r := range s {
		if strings.IndexRune(self.Separators, r) == -1 {
			return i, []int{utf8.RuneLen(r)}
		}
	}

	return -1, nil
}

func (self Single) String() string {
	return fmt.Sprintf("<single:![%s]>", self.Separators)
}
