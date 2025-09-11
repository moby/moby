package sliceutil_test

import (
	"net/netip"
	"strconv"
	"testing"

	"github.com/moby/moby/v2/internal/sliceutil"
)

func TestMap(t *testing.T) {
	s := []int{1, 2, 3}
	m := sliceutil.Map(s, func(i int) int { return i * 2 })
	if len(m) != len(s) {
		t.Errorf("len(m) = %d; want %d", len(m), len(s))
	}
	for i, v := range m {
		if expected := s[i] * 2; v != expected {
			t.Errorf("s[%d] = %d; want %d", i, expected, v)
		}
	}

	m = sliceutil.Map([]int(nil), func(i int) int { return i * 2 })
	if m != nil {
		t.Errorf("sliceutil.Map(nil, ...) = %v; want nil", m)
	}

	m = sliceutil.Map([]int{}, func(i int) int { return i * 2 })
	if m == nil || len(m) != 0 {
		t.Errorf("sliceutil.Map([], ...) = %v; want []", m)
	}
}

func TestMap_TypeConvert(t *testing.T) {
	s := []int{1, 2, 3}
	m := sliceutil.Map(s, func(i int) string { return strconv.Itoa(i) })
	if len(m) != len(s) {
		t.Fatalf("expected len %d, got %d", len(s), len(m))
	}
	for i, v := range m {
		if expected := strconv.Itoa(s[i]); v != expected {
			t.Fatalf("expected %s, got %s", expected, v)
		}
	}
}

func TestMapper(t *testing.T) {
	s := []string{"1.2.3.4", "fe80::1"}
	mapper := sliceutil.Mapper(netip.MustParseAddr)
	m := mapper(s)
	if len(m) != len(s) {
		t.Errorf("expected len %d, got %d", len(s), len(m))
	}
	for i, v := range m {
		if expected := netip.MustParseAddr(s[i]); v != expected {
			t.Errorf("expected %s, got %s", expected, v)
		}
	}

	if m := mapper(nil); m != nil {
		t.Errorf("mapper(nil) = %v; want nil", m)
	}
	if m := mapper([]string{}); m == nil || len(m) != 0 {
		t.Errorf("mapper([]) = %v; want []", m)
	}
}
