package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/volume/drivers"
	vt "github.com/docker/docker/volume/testutils"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestCreate(c *check.C) {
	volumedrivers.Register(vt.NewFakeDriver("fake"), "fake")
	defer volumedrivers.Unregister("fake")
	st, err := New("")
	if err != nil {
		c.Fatal(err)
	}
	v, err := st.Create("fake1", "fake", nil, nil)
	if err != nil {
		c.Fatal(err)
	}
	if v.Name() != "fake1" {
		c.Fatalf("Expected fake1 volume, got %v", v)
	}
	if l, _, _ := st.List(); len(l) != 1 {
		c.Fatalf("Expected 1 volume in the store, got %v: %v", len(l), l)
	}

	if _, err := st.Create("none", "none", nil, nil); err == nil {
		c.Fatalf("Expected unknown driver error, got nil")
	}

	_, err = st.Create("fakeerror", "fake", map[string]string{"error": "create error"}, nil)
	expected := &OpErr{Op: "create", Name: "fakeerror", Err: errors.New("create error")}
	if err != nil && err.Error() != expected.Error() {
		c.Fatalf("Expected create fakeError: create error, got %v", err)
	}
}

func (s *DockerSuite) TestRemove(c *check.C) {
	volumedrivers.Register(vt.NewFakeDriver("fake"), "fake")
	volumedrivers.Register(vt.NewFakeDriver("noop"), "noop")
	defer volumedrivers.Unregister("fake")
	defer volumedrivers.Unregister("noop")
	st, err := New("")
	if err != nil {
		c.Fatal(err)
	}

	// doing string compare here since this error comes directly from the driver
	expected := "no such volume"
	if err := st.Remove(vt.NoopVolume{}); err == nil || !strings.Contains(err.Error(), expected) {
		c.Fatalf("Expected error %q, got %v", expected, err)
	}

	v, err := st.CreateWithRef("fake1", "fake", "fake", nil, nil)
	if err != nil {
		c.Fatal(err)
	}

	if err := st.Remove(v); !IsInUse(err) {
		c.Fatalf("Expected ErrVolumeInUse error, got %v", err)
	}
	st.Dereference(v, "fake")
	if err := st.Remove(v); err != nil {
		c.Fatal(err)
	}
	if l, _, _ := st.List(); len(l) != 0 {
		c.Fatalf("Expected 0 volumes in the store, got %v, %v", len(l), l)
	}
}

func (s *DockerSuite) TestList(c *check.C) {
	volumedrivers.Register(vt.NewFakeDriver("fake"), "fake")
	volumedrivers.Register(vt.NewFakeDriver("fake2"), "fake2")
	defer volumedrivers.Unregister("fake")
	defer volumedrivers.Unregister("fake2")

	st, err := New("")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := st.Create("test", "fake", nil, nil); err != nil {
		c.Fatal(err)
	}
	if _, err := st.Create("test2", "fake2", nil, nil); err != nil {
		c.Fatal(err)
	}

	ls, _, err := st.List()
	if err != nil {
		c.Fatal(err)
	}
	if len(ls) != 2 {
		c.Fatalf("expected 2 volumes, got: %d", len(ls))
	}

	// and again with a new store
	st, err = New("")
	if err != nil {
		c.Fatal(err)
	}
	ls, _, err = st.List()
	if err != nil {
		c.Fatal(err)
	}
	if len(ls) != 2 {
		c.Fatalf("expected 2 volumes, got: %d", len(ls))
	}
}

func (s *DockerSuite) TestFilterByDriver(c *check.C) {
	volumedrivers.Register(vt.NewFakeDriver("fake"), "fake")
	volumedrivers.Register(vt.NewFakeDriver("noop"), "noop")
	defer volumedrivers.Unregister("fake")
	defer volumedrivers.Unregister("noop")
	st, err := New("")
	if err != nil {
		c.Fatal(err)
	}

	if _, err := st.Create("fake1", "fake", nil, nil); err != nil {
		c.Fatal(err)
	}
	if _, err := st.Create("fake2", "fake", nil, nil); err != nil {
		c.Fatal(err)
	}
	if _, err := st.Create("fake3", "noop", nil, nil); err != nil {
		c.Fatal(err)
	}

	if l, _ := st.FilterByDriver("fake"); len(l) != 2 {
		c.Fatalf("Expected 2 volumes, got %v, %v", len(l), l)
	}

	if l, _ := st.FilterByDriver("noop"); len(l) != 1 {
		c.Fatalf("Expected 1 volume, got %v, %v", len(l), l)
	}
}

func (s *DockerSuite) TestFilterByUsed(c *check.C) {
	volumedrivers.Register(vt.NewFakeDriver("fake"), "fake")
	volumedrivers.Register(vt.NewFakeDriver("noop"), "noop")
	defer volumedrivers.Unregister("fake")
	defer volumedrivers.Unregister("noop")

	st, err := New("")
	if err != nil {
		c.Fatal(err)
	}

	if _, err := st.CreateWithRef("fake1", "fake", "volReference", nil, nil); err != nil {
		c.Fatal(err)
	}
	if _, err := st.Create("fake2", "fake", nil, nil); err != nil {
		c.Fatal(err)
	}

	vols, _, err := st.List()
	if err != nil {
		c.Fatal(err)
	}

	dangling := st.FilterByUsed(vols, false)
	if len(dangling) != 1 {
		c.Fatalf("expected 1 danging volume, got %v", len(dangling))
	}
	if dangling[0].Name() != "fake2" {
		c.Fatalf("expected danging volume fake2, got %s", dangling[0].Name())
	}

	used := st.FilterByUsed(vols, true)
	if len(used) != 1 {
		c.Fatalf("expected 1 used volume, got %v", len(used))
	}
	if used[0].Name() != "fake1" {
		c.Fatalf("expected used volume fake1, got %s", used[0].Name())
	}
}

func (s *DockerSuite) TestDerefMultipleOfSameRef(c *check.C) {
	volumedrivers.Register(vt.NewFakeDriver("fake"), "fake")
	defer volumedrivers.Unregister("fake")

	st, err := New("")
	if err != nil {
		c.Fatal(err)
	}

	v, err := st.CreateWithRef("fake1", "fake", "volReference", nil, nil)
	if err != nil {
		c.Fatal(err)
	}

	if _, err := st.GetWithRef("fake1", "fake", "volReference"); err != nil {
		c.Fatal(err)
	}

	st.Dereference(v, "volReference")
	if err := st.Remove(v); err != nil {
		c.Fatal(err)
	}
}
