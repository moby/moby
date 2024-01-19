package mounts // import "github.com/docker/docker/volume/mounts"

import (
	"errors"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types/mount"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestValidateMount(t *testing.T) {
	testDir := t.TempDir()
	parser := NewParser()

	tests := []struct {
		input    mount.Mount
		expected error
	}{
		{
			input:    mount.Mount{Type: mount.TypeVolume},
			expected: errMissingField("Target"),
		},
		{
			input: mount.Mount{Type: mount.TypeVolume, Target: testDestinationPath, Source: "hello"},
		},
		{
			input: mount.Mount{Type: mount.TypeVolume, Target: testDestinationPath},
		},
		{
			input: mount.Mount{Type: mount.TypeVolume, Target: testDestinationPath, Source: "hello", VolumeOptions: &mount.VolumeOptions{Subpath: "world"}},
		},
		{
			input:    mount.Mount{Type: mount.TypeBind},
			expected: errMissingField("Target"),
		},
		{
			input:    mount.Mount{Type: mount.TypeBind, Target: testDestinationPath},
			expected: errMissingField("Source"),
		},
		{
			input:    mount.Mount{Type: mount.TypeBind, Target: testDestinationPath, Source: testSourcePath, VolumeOptions: &mount.VolumeOptions{}},
			expected: errExtraField("VolumeOptions"),
		},
		{
			input: mount.Mount{Type: mount.TypeBind, Source: testDir, Target: testDestinationPath},
		},
		{
			input:    mount.Mount{Type: "invalid", Target: testDestinationPath},
			expected: errors.New("mount type unknown"),
		},
		{
			input:    mount.Mount{Type: mount.TypeBind, Source: testSourcePath, Target: testDestinationPath},
			expected: errBindSourceDoesNotExist(testSourcePath),
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			err := parser.ValidateMountConfig(&tc.input)
			if tc.expected != nil {
				assert.Check(t, is.ErrorContains(err, tc.expected.Error()))
			} else {
				assert.Check(t, err)
			}
		})
	}
}

func TestValidateLCOWMount(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only tested on Windows")
	}
	testDir := t.TempDir()
	parser := NewLCOWParser()

	tests := []struct {
		input    mount.Mount
		expected error
	}{
		{
			input:    mount.Mount{Type: mount.TypeVolume},
			expected: errMissingField("Target"),
		},
		{
			input: mount.Mount{Type: mount.TypeVolume, Target: "/foo", Source: "hello"},
		},
		{
			input: mount.Mount{Type: mount.TypeVolume, Target: "/foo"},
		},
		{
			input:    mount.Mount{Type: mount.TypeBind},
			expected: errMissingField("Target"),
		},
		{
			input:    mount.Mount{Type: mount.TypeBind, Target: "/foo"},
			expected: errMissingField("Source"),
		},
		{
			input:    mount.Mount{Type: mount.TypeBind, Target: "/foo", Source: "c:\\foo", VolumeOptions: &mount.VolumeOptions{}},
			expected: errExtraField("VolumeOptions"),
		},
		{
			input:    mount.Mount{Type: mount.TypeBind, Source: "c:\\foo", Target: "/foo"},
			expected: errBindSourceDoesNotExist("c:\\foo"),
		},
		{
			input: mount.Mount{Type: mount.TypeBind, Source: testDir, Target: "/foo"},
		},
		{
			input:    mount.Mount{Type: "invalid", Target: "/foo"},
			expected: errors.New("mount type unknown"),
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			err := parser.ValidateMountConfig(&tc.input)
			if tc.expected != nil {
				assert.Check(t, is.ErrorContains(err, tc.expected.Error()))
			} else {
				assert.Check(t, err)
			}
		})
	}
}
