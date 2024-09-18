package sliceutil_test

import (
	"net/netip"
	"strconv"
	"testing"

	"github.com/docker/docker/internal/sliceutil"
)

func TestMap(t *testing.T) {
	s := []int{1, 2, 3}
	m := sliceutil.Map(s, func(i int) int { return i * 2 })
	if len(m) != len(s) {
		t.Fatalf("expected len %d, got %d", len(s), len(m))
	}
	for i, v := range m {
		if expected := s[i] * 2; v != expected {
			t.Fatalf("expected %d, got %d", expected, v)
		}
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
		t.Fatalf("expected len %d, got %d", len(s), len(m))
	}
	for i, v := range m {
		if expected := netip.MustParseAddr(s[i]); v != expected {
			t.Fatalf("expected %s, got %s", expected, v)
		}
	}
}
