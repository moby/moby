package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/volume/drivers"
	vt "github.com/docker/docker/volume/testutils"
)

func TestCreate(t *testing.T) {
	volumedrivers.Register(vt.NewFakeDriver("fake"), "fake")
	defer volumedrivers.Unregister("fake")
	s := New()
	v, err := s.Create("fake1", "fake", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Name() != "fake1" {
		t.Fatalf("Expected fake1 volume, got %v", v)
	}
	if l, _, _ := s.List(); len(l) != 1 {
		t.Fatalf("Expected 1 volume in the store, got %v: %v", len(l), l)
	}

	if _, err := s.Create("none", "none", nil); err == nil {
		t.Fatalf("Expected unknown driver error, got nil")
	}

	_, err = s.Create("fakeerror", "fake", map[string]string{"error": "create error"})
	expected := &OpErr{Op: "create", Name: "fakeerror", Err: errors.New("create error")}
	if err != nil && err.Error() != expected.Error() {
		t.Fatalf("Expected create fakeError: create error, got %v", err)
	}
}

func TestRemove(t *testing.T) {
	volumedrivers.Register(vt.NewFakeDriver("fake"), "fake")
	volumedrivers.Register(vt.NewFakeDriver("noop"), "noop")
	defer volumedrivers.Unregister("fake")
	defer volumedrivers.Unregister("noop")
	s := New()

	// doing string compare here since this error comes directly from the driver
	expected := "no such volume"
	if err := s.Remove(vt.NoopVolume{}); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Expected error %q, got %v", expected, err)
	}

	v, err := s.CreateWithRef("fake1", "fake", "fake", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Remove(v); !IsInUse(err) {
		t.Fatalf("Expected ErrVolumeInUse error, got %v", err)
	}
	s.Dereference(v, "fake")
	if err := s.Remove(v); err != nil {
		t.Fatal(err)
	}
	if l, _, _ := s.List(); len(l) != 0 {
		t.Fatalf("Expected 0 volumes in the store, got %v, %v", len(l), l)
	}
}

func TestList(t *testing.T) {
	volumedrivers.Register(vt.NewFakeDriver("fake"), "fake")
	volumedrivers.Register(vt.NewFakeDriver("fake2"), "fake2")
	defer volumedrivers.Unregister("fake")
	defer volumedrivers.Unregister("fake2")

	s := New()
	if _, err := s.Create("test", "fake", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("test2", "fake2", nil); err != nil {
		t.Fatal(err)
	}

	ls, _, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 2 {
		t.Fatalf("expected 2 volumes, got: %d", len(ls))
	}

	// and again with a new store
	s = New()
	ls, _, err = s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 2 {
		t.Fatalf("expected 2 volumes, got: %d", len(ls))
	}
}

func TestFilterByDriver(t *testing.T) {
	volumedrivers.Register(vt.NewFakeDriver("fake"), "fake")
	volumedrivers.Register(vt.NewFakeDriver("noop"), "noop")
	defer volumedrivers.Unregister("fake")
	defer volumedrivers.Unregister("noop")
	s := New()

	if _, err := s.Create("fake1", "fake", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("fake2", "fake", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("fake3", "noop", nil); err != nil {
		t.Fatal(err)
	}

	if l, _ := s.FilterByDriver("fake"); len(l) != 2 {
		t.Fatalf("Expected 2 volumes, got %v, %v", len(l), l)
	}

	if l, _ := s.FilterByDriver("noop"); len(l) != 1 {
		t.Fatalf("Expected 1 volume, got %v, %v", len(l), l)
	}
}
