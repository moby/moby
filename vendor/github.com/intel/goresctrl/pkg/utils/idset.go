/*
Copyright 2019-2021 Intel Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	// Unknown represents an unknown id.
	Unknown ID = -1
	// maxID sets an upper bound for the size of id sets
	// generated from listset strings, e.g. "0-16777215".
	maxID ID = (1 << 24) - 1
)

// ID is nn integer id, used to identify packages, CPUs, nodes, etc.
type ID = int

// IDSet is an unordered set of integer ids.
type IDSet map[ID]struct{}

// NewIDSet creates a new unordered set of (integer) ids.
func NewIDSet(ids ...ID) IDSet {
	s := make(map[ID]struct{})

	for _, id := range ids {
		s[id] = struct{}{}
	}

	return s
}

// NewIDSetFromIntSlice creates a new unordered set from an integer slice.
func NewIDSetFromIntSlice(ids ...int) IDSet {
	s := make(map[ID]struct{})

	for _, id := range ids {
		s[ID(id)] = struct{}{}
	}

	return s
}

// NewIDSetFromString creates new unordered set from string in listset syntax "0,61-63,2".
func NewIDSetFromString(listSet string) (IDSet, error) {
	s := NewIDSet()
	minValue := 0
	maxValue := maxID

	parts := strings.Split(listSet, ",")
	for _, part := range parts {
		switch {
		case part == "":
			continue
		case strings.Contains(part, "-"):
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, err
			}
			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, err
			}
			if start > end {
				return nil, fmt.Errorf("invalid range %s: start > end", part)
			}
			if start < minValue || end > maxValue {
				return nil, fmt.Errorf("invalid range %s: out of range %d-%d", part, minValue, maxValue)
			}
			for i := start; i <= end; i++ {
				s.Add(ID(i))
			}
		default:
			num, err := strconv.Atoi(part)
			if err != nil {
				return nil, err
			}
			if num < minValue || num > maxValue {
				return nil, fmt.Errorf("invalid value %d: out of range %d-%d", num, minValue, maxValue)
			}
			s.Add(ID(num))
		}
	}

	return s, nil
}

// Clone returns a copy of this IdSet.
func (s IDSet) Clone() IDSet {
	return NewIDSet(s.Members()...)
}

// Add adds the given ids into the set.
func (s IDSet) Add(ids ...ID) {
	for _, id := range ids {
		s[id] = struct{}{}
	}
}

// Del deletes the given ids from the set.
func (s IDSet) Del(ids ...ID) {
	if s != nil {
		for _, id := range ids {
			delete(s, id)
		}
	}
}

// Size returns the number of ids in the set.
func (s IDSet) Size() int {
	return len(s)
}

// Has tests if all the ids are present in the set.
func (s IDSet) Has(ids ...ID) bool {
	if s == nil {
		return false
	}

	for _, id := range ids {
		_, ok := s[id]
		if !ok {
			return false
		}
	}

	return true
}

// Members returns all ids in the set as a randomly ordered slice.
func (s IDSet) Members() []ID {
	if s == nil {
		return []ID{}
	}
	ids := make([]ID, len(s))
	idx := 0
	for id := range s {
		ids[idx] = id
		idx++
	}
	return ids
}

// SortedMembers returns all ids in the set as a sorted slice.
func (s IDSet) SortedMembers() []ID {
	ids := s.Members()
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}

// String returns the set as a string.
func (s IDSet) String() string {
	return s.StringWithSeparator(",")
}

// StringWithSeparator returns the set as a string, separated with the given separator.
func (s IDSet) StringWithSeparator(args ...string) string {
	if len(s) == 0 {
		return ""
	}

	var sep string

	if len(args) == 1 {
		sep = args[0]
	} else {
		sep = ","
	}

	str := ""
	t := ""
	for _, id := range s.SortedMembers() {
		str = str + t + strconv.Itoa(int(id))
		t = sep
	}

	return str
}

// MarshalJSON is the JSON marshaller for IDSet.
func (s IDSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON is the JSON unmarshaller for IDSet.
func (s *IDSet) UnmarshalJSON(data []byte) error {
	str := ""
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("invalid IDSet entry '%s': %v", string(data), err)
	}

	*s = NewIDSet()
	if str == "" {
		return nil
	}

	for _, idstr := range strings.Split(str, ",") {
		id, err := strconv.ParseInt(idstr, 10, 0)
		if err != nil {
			return fmt.Errorf("invalid IDSet entry '%s': %v", idstr, err)
		}
		s.Add(ID(id))
	}

	return nil
}
