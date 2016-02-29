package match

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// raw represents raw string to match
type Text struct {
	Str         string
	RunesLength int
	BytesLength int
}

func NewText(s string) Text {
	return Text{
		Str:         s,
		RunesLength: utf8.RuneCountInString(s),
		BytesLength: len(s),
	}
}

func (self Text) Match(s string) bool {
	return self.Str == s
}

func (self Text) Len() int {
	return self.RunesLength
}

func (self Text) Index(s string) (index int, segments []int) {
	index = strings.Index(s, self.Str)
	if index == -1 {
		return
	}

	segments = []int{self.BytesLength}

	return
}

func (self Text) String() string {
	return fmt.Sprintf("<text:%s>", self.Str)
}
