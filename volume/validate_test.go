package volume

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/engine-api/types/container"
)

func TestValidateMount(t *testing.T) {
	testDir, err := ioutil.TempDir("", "test-validate-mount")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testDir)

	cases := []struct {
		input    container.MountConfig
		expected error
	}{
		{container.MountConfig{Type: MountTypeEphemeral}, fmt.Errorf("Invalid mount spec: Destination must not be empty")},
		{container.MountConfig{Type: MountTypeEphemeral, Destination: "/foo", Source: "/foo"}, fmt.Errorf("Invalid ephemeral mount spec: Source must not be specified")},
		{container.MountConfig{Type: MountTypeEphemeral, Name: "hello", Destination: "/foo"}, fmt.Errorf("Invalid ephemeral mount spec: Name must not be specified")},
		{container.MountConfig{Type: MountTypeEphemeral, Destination: "/foo"}, nil},
		{container.MountConfig{Type: MountTypeHostBind}, fmt.Errorf("Invalid mount spec: Destination must not be empty")},
		{container.MountConfig{Type: MountTypeHostBind, Destination: "/foo", Name: "hello"}, fmt.Errorf("Invalid hostbind mount spec: Name must not be specified")},
		{container.MountConfig{Type: MountTypeHostBind, Destination: "/foo"}, fmt.Errorf("Invalid hostbind mount spec: Source must not be empty")},
		{container.MountConfig{Type: MountTypeHostBind, Source: "/foo", Destination: "/foo", Driver: "whatevs"}, fmt.Errorf("Invalid hostbind mount spec: Driver must not be specified")},
		{container.MountConfig{Type: MountTypeHostBind, Source: "/non-existent", Destination: "/foo"}, fmt.Errorf("Invalid hostbind mount spec for Source: path does not exist")},
		{container.MountConfig{Type: MountTypeHostBind, Source: testDir, Destination: "/foo"}, nil},
		{container.MountConfig{Type: MountTypePersistent}, fmt.Errorf("Invalid mount spec: Destination must not be empty")},
		{container.MountConfig{Type: MountTypePersistent, Destination: "/foo", Name: "hello"}, nil},
		{container.MountConfig{Type: MountTypePersistent, Destination: "/foo", Source: "/foo", Name: "hello"}, fmt.Errorf("Invalid ephemeral mount spec: Source must not be specified")},
		{container.MountConfig{Type: MountTypeEphemeral, Destination: "/foo", Mode: "not-real"}, fmt.Errorf("Invalid mount spec: Mode \"not-real\" is invalid")},
		{container.MountConfig{Type: MountTypeEphemeral, Destination: "/foo", Mode: "rw"}, nil},
		{container.MountConfig{Type: "invalid", Destination: "/foo", Mode: "rw"}, fmt.Errorf("Invalid mount spec: mount type unknown: \"invalid\"")},
	}
	for i, x := range cases {
		err := validateMountConfig(&x.input)
		if err == nil && x.expected == nil {
			continue
		}
		if (err == nil && x.expected != nil) || (x.expected == nil && err != nil) || (err.Error() != x.expected.Error()) {
			t.Fatalf("expected %q, got %q, case: %d", x.expected, err, i)
		}
	}

}
