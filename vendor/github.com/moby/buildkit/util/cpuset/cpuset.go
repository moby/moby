// Package cpuset parses and formats cpuset strings like "0-3,5,7-9".
package cpuset

import (
	"slices"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// MaxCPU is the largest CPU/memory-node index Parse accepts. It bounds the set
// Parse allocates so an untrusted value like "0-999999999" cannot exhaust memory.
const MaxCPU = 8192

// Parse parses a cpuset string into a set of integers. An empty input returns
// an empty set with no error. Elements must be in the range [0, MaxCPU].
func Parse(s string) (map[int]struct{}, error) {
	out := make(map[int]struct{})
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lo, hi, found := strings.Cut(part, "-")
		if !found {
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 {
				return nil, errors.Errorf("invalid cpuset element %q", part)
			}
			if n > MaxCPU {
				return nil, errors.Errorf("cpuset element %q exceeds maximum %d", part, MaxCPU)
			}
			out[n] = struct{}{}
			continue
		}
		loN, err1 := strconv.Atoi(strings.TrimSpace(lo))
		hiN, err2 := strconv.Atoi(strings.TrimSpace(hi))
		if err1 != nil || err2 != nil || loN < 0 || hiN < loN {
			return nil, errors.Errorf("invalid cpuset range %q", part)
		}
		if hiN > MaxCPU {
			return nil, errors.Errorf("cpuset range %q exceeds maximum %d", part, MaxCPU)
		}
		for i := loN; i <= hiN; i++ {
			out[i] = struct{}{}
		}
	}
	return out, nil
}

// Validate returns an error if s is not a syntactically valid cpuset string.
// An empty string is valid (means "unset").
func Validate(s string) error {
	_, err := Parse(s)
	return err
}

// Format emits a sorted, range-collapsed cpuset string from a set.
func Format(set map[int]struct{}) string {
	if len(set) == 0 {
		return ""
	}
	vals := make([]int, 0, len(set))
	for v := range set {
		vals = append(vals, v)
	}
	slices.Sort(vals)

	var sb strings.Builder
	i := 0
	for i < len(vals) {
		j := i
		for j+1 < len(vals) && vals[j+1] == vals[j]+1 {
			j++
		}
		if sb.Len() > 0 {
			sb.WriteByte(',')
		}
		if i == j {
			sb.WriteString(strconv.Itoa(vals[i]))
		} else {
			sb.WriteString(strconv.Itoa(vals[i]))
			sb.WriteByte('-')
			sb.WriteString(strconv.Itoa(vals[j]))
		}
		i = j + 1
	}
	return sb.String()
}
