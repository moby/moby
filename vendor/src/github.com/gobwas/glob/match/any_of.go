package match

import (
	"fmt"
)

type AnyOf struct {
	Matchers Matchers
}

func (self *AnyOf) Add(m Matcher) error {
	self.Matchers = append(self.Matchers, m)
	return nil
}

func (self AnyOf) Match(s string) bool {
	for _, m := range self.Matchers {
		if m.Match(s) {
			return true
		}
	}

	return false
}

func (self AnyOf) Index(s string) (int, []int) {
	if len(self.Matchers) == 0 {
		return -1, nil
	}

	// segments to merge
	var segments [][]int
	index := -1

	for _, m := range self.Matchers {
		idx, seg := m.Index(s)
		if idx == -1 {
			continue
		}

		if index == -1 || idx < index {
			index = idx
			segments = [][]int{seg}
			continue
		}

		if idx > index {
			continue
		}

		segments = append(segments, seg)
	}

	if index == -1 {
		return -1, nil
	}

	return index, mergeSegments(segments)
}

func (self AnyOf) Len() (l int) {
	l = -1
	for _, m := range self.Matchers {
		ml := m.Len()
		if ml == -1 {
			return -1
		}

		if l == -1 {
			l = ml
			continue
		}

		if l != ml {
			return -1
		}
	}

	return
}

func (self AnyOf) String() string {
	return fmt.Sprintf("<any_of:[%s]>", self.Matchers)
}
