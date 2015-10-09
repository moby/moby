package store

import (
	"strings"
	"testing"

	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
	vt "github.com/docker/docker/volume/testutils"
)

func TestList(t *testing.T) {
	volumedrivers.Register(vt.FakeDriver{}, "fake")
	s := New()
	s.AddAll([]volume.Volume{vt.NewFakeVolume("fake1"), vt.NewFakeVolume("fake2")})
	l := s.List()
	if len(l) != 2 {
		t.Fatalf("Expected 2 volumes in the store, got %v: %v", len(l), l)
	}
}

func TestGet(t *testing.T) {
	volumedrivers.Register(vt.FakeDriver{}, "fake")
	s := New()
	s.AddAll([]volume.Volume{vt.NewFakeVolume("fake1"), vt.NewFakeVolume("fake2")})
	v, err := s.Get("fake1")
	if err != nil {
		t.Fatal(err)
	}
	if v.Name() != "fake1" {
		t.Fatalf("Expected fake1 volume, got %v", v)
	}

	if _, err := s.Get("fake4"); err != ErrNoSuchVolume {
		t.Fatalf("Expected ErrNoSuchVolume error, got %v", err)
	}
}

func TestCreate(t *testing.T) {
	volumedrivers.Register(vt.FakeDriver{}, "fake")
	s := New()
	v, err := s.Create("fake1", "fake", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Name() != "fake1" {
		t.Fatalf("Expected fake1 volume, got %v", v)
	}
	if l := s.List(); len(l) != 1 {
		t.Fatalf("Expected 1 volume in the store, got %v: %v", len(l), l)
	}

	if _, err := s.Create("none", "none", nil); err == nil {
		t.Fatalf("Expected unknown driver error, got nil")
	}

	_, err = s.Create("fakeError", "fake", map[string]string{"error": "create error"})
	if err == nil || err.Error() != "create error" {
		t.Fatalf("Expected create error, got %v", err)
	}
}

func TestRemove(t *testing.T) {
	volumedrivers.Register(vt.FakeDriver{}, "fake")
	s := New()
	if err := s.Remove(vt.NoopVolume{}); err != ErrNoSuchVolume {
		t.Fatalf("Expected ErrNoSuchVolume error, got %v", err)
	}
	v, err := s.Create("fake1", "fake", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Increment(v, "testvolume01")
	if err := s.Remove(v); !strings.Contains(err.Error(), ErrVolumeInUse.Error()) {
		t.Fatalf("Expected ErrVolumeInUse error, got %v", err)
	}
	s.Decrement(v, "testvolume02")
	if err := s.Remove(v); err != nil {
		t.Fatal(err)
	}
	if l := s.List(); len(l) != 0 {
		t.Fatalf("Expected 0 volumes in the store, got %v, %v", len(l), l)
	}
}

func TestIncrement(t *testing.T) {
	s := New()
	v := vt.NewFakeVolume("fake1")
	s.Increment(v, "testvolume01")
	if l := s.List(); len(l) != 1 {
		t.Fatalf("Expected 1 volume, got %v, %v", len(l), l)
	}
	if c := s.Count(v); c != 1 {
		t.Fatalf("Expected 1 counter, got %v", c)
	}

	s.Increment(v, "testvolume02")
	if l := s.List(); len(l) != 1 {
		t.Fatalf("Expected 1 volume, got %v, %v", len(l), l)
	}
	if c := s.Count(v); c != 2 {
		t.Fatalf("Expected 2 counter, got %v", c)
	}

	v2 := vt.NewFakeVolume("fake2")
	s.Increment(v2, "testvolume03")
	if l := s.List(); len(l) != 2 {
		t.Fatalf("Expected 2 volume, got %v, %v", len(l), l)
	}
}

func TestDecrement(t *testing.T) {
	s := New()
	v := vt.NoopVolume{}
	s.Decrement(v, "testvolume01")
	if c := s.Count(v); c != 0 {
		t.Fatalf("Expected 0 volumes, got %v", c)
	}

	s.Increment(v, "testvolume02")
	s.Increment(v, "testvolume03")
	s.Decrement(v, "testvolume02")
	if c := s.Count(v); c != 1 {
		t.Fatalf("Expected 1 volume, got %v", c)
	}

	s.Decrement(v, "testvolume03")
	if c := s.Count(v); c != 0 {
		t.Fatalf("Expected 0 volumes, got %v", c)
	}

	// Test counter cannot be negative.
	s.Decrement(v, "testvolume01")
	if c := s.Count(v); c != 0 {
		t.Fatalf("Expected 0 volumes, got %v", c)
	}
}

func TestFilterByDriver(t *testing.T) {
	s := New()

	s.Increment(vt.NewFakeVolume("fake1"), "testvolume01")
	s.Increment(vt.NewFakeVolume("fake2"), "testvolume02")
	s.Increment(vt.NoopVolume{}, "testvolume03")

	if l := s.FilterByDriver("fake"); len(l) != 2 {
		t.Fatalf("Expected 2 volumes, got %v, %v", len(l), l)
	}

	if l := s.FilterByDriver("noop"); len(l) != 1 {
		t.Fatalf("Expected 1 volume, got %v, %v", len(l), l)
	}
}
