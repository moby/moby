package service

import (
	"strings"
	"testing"
	"time"

	"github.com/docker/engine-api/types/swarm"
)

func assertEqual(t *testing.T, actual, expected interface{}) {
	if expected != actual {
		t.Fatalf("Expected '%v' (%T) got '%v' (%T)", expected, expected, actual, actual)
	}
}

func assertNilError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("Expected no error, got: %s", err.Error())
	}
}

func assertError(t *testing.T, err error, contains string) {
	if err == nil {
		t.Fatalf("Expected an error, but error was nil")
	}

	if !strings.Contains(err.Error(), contains) {
		t.Fatalf("Expected error to contain '%s', got '%s'", contains, err.Error())
	}
}

func TestMemBytesString(t *testing.T) {
	var mem memBytes = 1048576
	assertEqual(t, mem.String(), "1 MiB")
}

func TestMemBytesSetAndValue(t *testing.T) {
	var mem memBytes
	assertNilError(t, mem.Set("5kb"))
	assertEqual(t, mem.Value(), int64(5120))
}

func TestNanoCPUsString(t *testing.T) {
	var cpus nanoCPUs = 6100000000
	assertEqual(t, cpus.String(), "6.100")
}

func TestNanoCPUsSetAndValue(t *testing.T) {
	var cpus nanoCPUs
	assertNilError(t, cpus.Set("0.35"))
	assertEqual(t, cpus.Value(), int64(350000000))
}

func TestDurationOptString(t *testing.T) {
	dur := time.Duration(300 * 10e8)
	duration := DurationOpt{value: &dur}
	assertEqual(t, duration.String(), "5m0s")
}

func TestDurationOptSetAndValue(t *testing.T) {
	var duration DurationOpt
	assertNilError(t, duration.Set("300s"))
	assertEqual(t, *duration.Value(), time.Duration(300*10e8))
}

func TestUint64OptString(t *testing.T) {
	value := uint64(2345678)
	opt := Uint64Opt{value: &value}
	assertEqual(t, opt.String(), "2345678")

	opt = Uint64Opt{}
	assertEqual(t, opt.String(), "none")
}

func TestUint64OptSetAndValue(t *testing.T) {
	var opt Uint64Opt
	assertNilError(t, opt.Set("14445"))
	assertEqual(t, *opt.Value(), uint64(14445))
}

func TestMountOptString(t *testing.T) {
	mount := MountOpt{
		values: []swarm.Mount{
			{
				Type:   swarm.MountType("BIND"),
				Source: "/home/path",
				Target: "/target",
			},
			{
				Type:   swarm.MountType("VOLUME"),
				Source: "foo",
				Target: "/target/foo",
			},
		},
	}
	expected := "BIND /home/path /target, VOLUME foo /target/foo"
	assertEqual(t, mount.String(), expected)
}

func TestMountOptSetNoError(t *testing.T) {
	var mount MountOpt
	assertNilError(t, mount.Set("type=bind,target=/target,source=/foo"))

	mounts := mount.Value()
	assertEqual(t, len(mounts), 1)
	assertEqual(t, mounts[0], swarm.Mount{
		Type:   swarm.MountType("BIND"),
		Source: "/foo",
		Target: "/target",
	})
}

func TestMountOptSetErrorNoType(t *testing.T) {
	var mount MountOpt
	assertError(t, mount.Set("target=/target,source=/foo"), "type is required")
}

func TestMountOptSetErrorNoTarget(t *testing.T) {
	var mount MountOpt
	assertError(t, mount.Set("type=VOLUME,source=/foo"), "target is required")
}

func TestMountOptSetErrorInvalidKey(t *testing.T) {
	var mount MountOpt
	assertError(t, mount.Set("type=VOLUME,bogus=foo"), "unexpected key 'bogus'")
}

func TestMountOptSetErrorInvalidField(t *testing.T) {
	var mount MountOpt
	assertError(t, mount.Set("type=VOLUME,bogus"), "invalid field 'bogus'")
}

func TestMountOptSetErrorInvalidWritable(t *testing.T) {
	var mount MountOpt
	assertError(t, mount.Set("type=VOLUME,writable=yes"), "invalid value for writable: yes")
}
