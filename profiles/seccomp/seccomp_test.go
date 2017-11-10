// +build linux

package seccomp

import (
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/docker/docker/oci"
)

func TestLoadProfile(t *testing.T) {
	f, err := ioutil.ReadFile("fixtures/example.json")
	if err != nil {
		t.Fatal(err)
	}
	rs := oci.DefaultSpec()
	if _, err := LoadProfile(string(f), &rs); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaultProfile(t *testing.T) {
	f, err := ioutil.ReadFile("default.json")
	if err != nil {
		t.Fatal(err)
	}
	rs := oci.DefaultSpec()
	if _, err := LoadProfile(string(f), &rs); err != nil {
		t.Fatal(err)
	}
}

func TestGetSeccompArch(t *testing.T) {
	if _, err := getSeccompArch(); err != nil {
		t.Error(err)
	}
}

func TestSupportLegacyArchID(t *testing.T) {
	f := func(input, expected []string) {
		output := supportLegacyArchID(input)
		if !reflect.DeepEqual(expected, output) {
			t.Errorf("%v != %v <= %v", expected, output, input)
		}
	}

	f([]string{"SCMP_ARCH_X86", "SCMP_ARCH_X86_64"},
		[]string{"SCMP_ARCH_X86", "SCMP_ARCH_X86_64"})
	f([]string{}, []string{})
	f([]string{"arm64", "SCMP_ARCH_X86", "SCMP_ARCH_X86_64"},
		[]string{"SCMP_ARCH_AARCH64", "SCMP_ARCH_X86", "SCMP_ARCH_X86_64"})
	f([]string{"SCMP_ARCH_X86", "arm64", "SCMP_ARCH_X86_64"},
		[]string{"SCMP_ARCH_X86", "SCMP_ARCH_AARCH64", "SCMP_ARCH_X86_64"})
	f([]string{"SCMP_ARCH_X86", "SCMP_ARCH_X86_64", "arm64"},
		[]string{"SCMP_ARCH_X86", "SCMP_ARCH_X86_64", "SCMP_ARCH_AARCH64"})
	f([]string{"SCMP_ARCH_X86", "SCMP_ARCH_X86_64", "arm64", "s390"},
		[]string{"SCMP_ARCH_X86", "SCMP_ARCH_X86_64", "SCMP_ARCH_AARCH64", "SCMP_ARCH_S390"})
}
