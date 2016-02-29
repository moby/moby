package match

import (
	"fmt"
)

type EveryOf struct {
	Matchers Matchers
}

func (self *EveryOf) Add(m Matcher) error {
	self.Matchers = append(self.Matchers, m)
	return nil
}

func (self EveryOf) Len() (l int) {
	for _, m := range self.Matchers {
		if ml := m.Len(); l > 0 {
			l += ml
		} else {
			return -1
		}
	}

	return
}

func (self EveryOf) Index(s string) (int, []int) {
	var index int
	var offset int
	var segments []int

	sub := s
	for _, m := range self.Matchers {
		idx, seg := m.Index(sub)
		if idx == -1 {
			return -1, nil
		}

		var sum []int
		if segments == nil {
			sum = seg
		} else {
			delta := index - (idx + offset)
			for _, ex := range segments {
				for _, n := range seg {
					if ex+delta == n {
						sum = append(sum, n)
					}
				}
			}
		}

		if len(sum) == 0 {
			return -1, nil
		}

		segments = sum
		index = idx + offset
		sub = s[index:]
		offset += idx
	}

	return index, segments
}

func (self EveryOf) Match(s string) bool {
	for _, m := range self.Matchers {
		if !m.Match(s) {
			return false
		}
	}

	return true
}

func (self EveryOf) String() string {
	return fmt.Sprintf("<every_of:[%s]>", self.Matchers)
}
