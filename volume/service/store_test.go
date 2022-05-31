package service // import "github.com/docker/docker/volume/service"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/volume"
	volumedrivers "github.com/docker/docker/volume/drivers"
	"github.com/docker/docker/volume/service/opts"
	volumetestutils "github.com/docker/docker/volume/testutils"
	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	s, cleanup := setupTest(t)
	defer cleanup()
	s.drivers.Register(volumetestutils.NewFakeDriver("fake"), "fake")

	ctx := context.Background()
	v, err := s.Create(ctx, "fake1", "fake")
	if err != nil {
		t.Fatal(err)
	}
	if v.Name() != "fake1" {
		t.Fatalf("Expected fake1 volume, got %v", v)
	}
	if l, _, _ := s.Find(ctx, nil); len(l) != 1 {
		t.Fatalf("Expected 1 volume in the store, got %v: %v", len(l), l)
	}

	if _, err := s.Create(ctx, "none", "none"); err == nil {
		t.Fatalf("Expected unknown driver error, got nil")
	}

	_, err = s.Create(ctx, "fakeerror", "fake", opts.WithCreateOptions(map[string]string{"error": "create error"}))
	expected := &OpErr{Op: "create", Name: "fakeerror", Err: errors.New("create error")}
	if err != nil && err.Error() != expected.Error() {
		t.Fatalf("Expected create fakeError: create error, got %v", err)
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	s, cleanup := setupTest(t)
	defer cleanup()

	s.drivers.Register(volumetestutils.NewFakeDriver("fake"), "fake")
	s.drivers.Register(volumetestutils.NewFakeDriver("noop"), "noop")

	ctx := context.Background()

	// doing string compare here since this error comes directly from the driver
	expected := "no such volume"
	var v volume.Volume = volumetestutils.NoopVolume{}
	if err := s.Remove(ctx, v); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Expected error %q, got %v", expected, err)
	}

	v, err := s.Create(ctx, "fake1", "fake", opts.WithCreateReference("fake"))
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Remove(ctx, v); !IsInUse(err) {
		t.Fatalf("Expected ErrVolumeInUse error, got %v", err)
	}
	s.Release(ctx, v.Name(), "fake")
	if err := s.Remove(ctx, v); err != nil {
		t.Fatal(err)
	}
	if l, _, _ := s.Find(ctx, nil); len(l) != 0 {
		t.Fatalf("Expected 0 volumes in the store, got %v, %v", len(l), l)
	}
}

func TestList(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp("", "test-list")
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	drivers := volumedrivers.NewStore(nil)
	drivers.Register(volumetestutils.NewFakeDriver("fake"), "fake")
	drivers.Register(volumetestutils.NewFakeDriver("fake2"), "fake2")

	s, err := NewStore(dir, drivers)
	assert.NilError(t, err)

	ctx := context.Background()
	if _, err := s.Create(ctx, "test", "fake"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "test2", "fake2"); err != nil {
		t.Fatal(err)
	}

	ls, _, err := s.Find(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 2 {
		t.Fatalf("expected 2 volumes, got: %d", len(ls))
	}
	if err := s.Shutdown(); err != nil {
		t.Fatal(err)
	}

	// and again with a new store
	s, err = NewStore(dir, drivers)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Shutdown()
	ls, _, err = s.Find(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 2 {
		t.Fatalf("expected 2 volumes, got: %d", len(ls))
	}
}

func TestFindByDriver(t *testing.T) {
	t.Parallel()
	s, cleanup := setupTest(t)
	defer cleanup()

	assert.Assert(t, s.drivers.Register(volumetestutils.NewFakeDriver("fake"), "fake"))
	assert.Assert(t, s.drivers.Register(volumetestutils.NewFakeDriver("noop"), "noop"))

	ctx := context.Background()
	_, err := s.Create(ctx, "fake1", "fake")
	assert.NilError(t, err)

	_, err = s.Create(ctx, "fake2", "fake")
	assert.NilError(t, err)

	_, err = s.Create(ctx, "fake3", "noop")
	assert.NilError(t, err)

	l, _, err := s.Find(ctx, ByDriver("fake"))
	assert.NilError(t, err)
	assert.Equal(t, len(l), 2)

	l, _, err = s.Find(ctx, ByDriver("noop"))
	assert.NilError(t, err)
	assert.Equal(t, len(l), 1)

	l, _, err = s.Find(ctx, ByDriver("nosuchdriver"))
	assert.NilError(t, err)
	assert.Equal(t, len(l), 0)
}

func TestFindByReferenced(t *testing.T) {
	t.Parallel()
	s, cleanup := setupTest(t)
	defer cleanup()

	s.drivers.Register(volumetestutils.NewFakeDriver("fake"), "fake")
	s.drivers.Register(volumetestutils.NewFakeDriver("noop"), "noop")

	ctx := context.Background()
	if _, err := s.Create(ctx, "fake1", "fake", opts.WithCreateReference("volReference")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "fake2", "fake"); err != nil {
		t.Fatal(err)
	}

	dangling, _, err := s.Find(ctx, ByReferenced(false))
	assert.NilError(t, err)
	assert.Assert(t, len(dangling) == 1)
	assert.Check(t, dangling[0].Name() == "fake2")

	used, _, err := s.Find(ctx, ByReferenced(true))
	assert.NilError(t, err)
	assert.Assert(t, len(used) == 1)
	assert.Check(t, used[0].Name() == "fake1")
}

func TestDerefMultipleOfSameRef(t *testing.T) {
	t.Parallel()
	s, cleanup := setupTest(t)
	defer cleanup()
	s.drivers.Register(volumetestutils.NewFakeDriver("fake"), "fake")

	ctx := context.Background()
	v, err := s.Create(ctx, "fake1", "fake", opts.WithCreateReference("volReference"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Get(ctx, "fake1", opts.WithGetDriver("fake"), opts.WithGetReference("volReference")); err != nil {
		t.Fatal(err)
	}

	s.Release(ctx, v.Name(), "volReference")
	if err := s.Remove(ctx, v); err != nil {
		t.Fatal(err)
	}
}

func TestCreateKeepOptsLabelsWhenExistsRemotely(t *testing.T) {
	t.Parallel()
	s, cleanup := setupTest(t)
	defer cleanup()

	vd := volumetestutils.NewFakeDriver("fake")
	s.drivers.Register(vd, "fake")

	// Create a volume in the driver directly
	if _, err := vd.Create("foo", nil); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	v, err := s.Create(ctx, "foo", "fake", opts.WithCreateLabels(map[string]string{"hello": "world"}))
	if err != nil {
		t.Fatal(err)
	}

	switch dv := v.(type) {
	case volume.DetailedVolume:
		if dv.Labels()["hello"] != "world" {
			t.Fatalf("labels don't match")
		}
	default:
		t.Fatalf("got unexpected type: %T", v)
	}
}

func TestDefererencePluginOnCreateError(t *testing.T) {
	t.Parallel()

	var (
		l   net.Listener
		err error
	)

	for i := 32768; l == nil && i < 40000; i++ {
		l, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", i))
	}
	if l == nil {
		t.Fatalf("could not create listener: %v", err)
	}
	defer l.Close()

	s, cleanup := setupTest(t)
	defer cleanup()

	d := volumetestutils.NewFakeDriver("TestDefererencePluginOnCreateError")
	p, err := volumetestutils.MakeFakePlugin(d, l)
	if err != nil {
		t.Fatal(err)
	}

	pg := volumetestutils.NewFakePluginGetter(p)
	s.drivers = volumedrivers.NewStore(pg)

	ctx := context.Background()
	// create a good volume so we have a plugin reference
	_, err = s.Create(ctx, "fake1", d.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Now create another one expecting an error
	_, err = s.Create(ctx, "fake2", d.Name(), opts.WithCreateOptions(map[string]string{"error": "some error"}))
	if err == nil || !strings.Contains(err.Error(), "some error") {
		t.Fatalf("expected an error on create: %v", err)
	}

	// There should be only 1 plugin reference
	if refs := volumetestutils.FakeRefs(p); refs != 1 {
		t.Fatalf("expected 1 plugin reference, got: %d", refs)
	}
}

func TestRefDerefRemove(t *testing.T) {
	t.Parallel()

	driverName := "test-ref-deref-remove"
	s, cleanup := setupTest(t)
	defer cleanup()
	s.drivers.Register(volumetestutils.NewFakeDriver(driverName), driverName)

	ctx := context.Background()
	v, err := s.Create(ctx, "test", driverName, opts.WithCreateReference("test-ref"))
	assert.NilError(t, err)

	err = s.Remove(ctx, v)
	assert.Assert(t, is.ErrorContains(err, ""))
	assert.Equal(t, errVolumeInUse, err.(*OpErr).Err)

	s.Release(ctx, v.Name(), "test-ref")
	err = s.Remove(ctx, v)
	assert.NilError(t, err)
}

func TestGet(t *testing.T) {
	t.Parallel()

	driverName := "test-get"
	s, cleanup := setupTest(t)
	defer cleanup()
	s.drivers.Register(volumetestutils.NewFakeDriver(driverName), driverName)

	ctx := context.Background()
	_, err := s.Get(ctx, "not-exist")
	assert.Assert(t, is.ErrorContains(err, ""))
	assert.Equal(t, errNoSuchVolume, err.(*OpErr).Err)

	v1, err := s.Create(ctx, "test", driverName, opts.WithCreateLabels(map[string]string{"a": "1"}))
	assert.NilError(t, err)

	v2, err := s.Get(ctx, "test")
	assert.NilError(t, err)
	assert.DeepEqual(t, v1, v2, cmpVolume)

	dv := v2.(volume.DetailedVolume)
	assert.Equal(t, "1", dv.Labels()["a"])

	err = s.Remove(ctx, v1)
	assert.NilError(t, err)
}

func TestGetWithReference(t *testing.T) {
	t.Parallel()

	driverName := "test-get-with-ref"
	s, cleanup := setupTest(t)
	defer cleanup()
	s.drivers.Register(volumetestutils.NewFakeDriver(driverName), driverName)

	ctx := context.Background()
	_, err := s.Get(ctx, "not-exist", opts.WithGetDriver(driverName), opts.WithGetReference("test-ref"))
	assert.Assert(t, is.ErrorContains(err, ""))

	v1, err := s.Create(ctx, "test", driverName, opts.WithCreateLabels(map[string]string{"a": "1"}))
	assert.NilError(t, err)

	v2, err := s.Get(ctx, "test", opts.WithGetDriver(driverName), opts.WithGetReference("test-ref"))
	assert.NilError(t, err)
	assert.DeepEqual(t, v1, v2, cmpVolume)

	err = s.Remove(ctx, v2)
	assert.Assert(t, is.ErrorContains(err, ""))
	assert.Equal(t, errVolumeInUse, err.(*OpErr).Err)

	s.Release(ctx, v2.Name(), "test-ref")
	err = s.Remove(ctx, v2)
	assert.NilError(t, err)
}

var cmpVolume = cmp.AllowUnexported(volumetestutils.FakeVolume{}, volumeWrapper{})

func setupTest(t *testing.T) (*VolumeStore, func()) {
	t.Helper()

	dirName := strings.ReplaceAll(t.Name(), string(os.PathSeparator), "_")
	dir, err := os.MkdirTemp("", dirName)
	assert.NilError(t, err)

	cleanup := func() {
		t.Helper()
		err := os.RemoveAll(dir)
		assert.Check(t, err)
	}

	s, err := NewStore(dir, volumedrivers.NewStore(nil))
	assert.Check(t, err)
	return s, func() {
		s.Shutdown()
		cleanup()
	}
}

func TestFilterFunc(t *testing.T) {
	testDriver := volumetestutils.NewFakeDriver("test")
	testVolume, err := testDriver.Create("test", nil)
	assert.NilError(t, err)
	testVolume2, err := testDriver.Create("test2", nil)
	assert.NilError(t, err)
	testVolume3, err := testDriver.Create("test3", nil)
	assert.NilError(t, err)

	for _, test := range []struct {
		vols   []volume.Volume
		fn     filterFunc
		desc   string
		expect []volume.Volume
	}{
		{desc: "test nil list", vols: nil, expect: nil, fn: func(volume.Volume) bool { return true }},
		{desc: "test empty list", vols: []volume.Volume{}, expect: []volume.Volume{}, fn: func(volume.Volume) bool { return true }},
		{desc: "test filter non-empty to empty", vols: []volume.Volume{testVolume}, expect: []volume.Volume{}, fn: func(volume.Volume) bool { return false }},
		{desc: "test nothing to fitler non-empty list", vols: []volume.Volume{testVolume}, expect: []volume.Volume{testVolume}, fn: func(volume.Volume) bool { return true }},
		{desc: "test filter some", vols: []volume.Volume{testVolume, testVolume2}, expect: []volume.Volume{testVolume}, fn: func(v volume.Volume) bool { return v.Name() == testVolume.Name() }},
		{desc: "test filter middle", vols: []volume.Volume{testVolume, testVolume2, testVolume3}, expect: []volume.Volume{testVolume, testVolume3}, fn: func(v volume.Volume) bool { return v.Name() != testVolume2.Name() }},
		{desc: "test filter middle and last", vols: []volume.Volume{testVolume, testVolume2, testVolume3}, expect: []volume.Volume{testVolume}, fn: func(v volume.Volume) bool { return v.Name() != testVolume2.Name() && v.Name() != testVolume3.Name() }},
		{desc: "test filter first and last", vols: []volume.Volume{testVolume, testVolume2, testVolume3}, expect: []volume.Volume{testVolume2}, fn: func(v volume.Volume) bool { return v.Name() != testVolume.Name() && v.Name() != testVolume3.Name() }},
	} {
		t.Run(test.desc, func(t *testing.T) {
			test := test
			t.Parallel()

			filter(&test.vols, test.fn)
			assert.DeepEqual(t, test.vols, test.expect, cmpVolume)
		})
	}
}
