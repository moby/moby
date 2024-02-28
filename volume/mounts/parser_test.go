package mounts // import "github.com/docker/docker/volume/mounts"

import (
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/mount"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type parseMountRawTestSet struct {
	valid   []string
	invalid map[string]string
}

func TestConvertTmpfsOptions(t *testing.T) {
	type testCase struct {
		opt                  mount.TmpfsOptions
		readOnly             bool
		expectedSubstrings   []string
		unexpectedSubstrings []string
		err                  bool
	}
	cases := []testCase{
		{
			opt:                  mount.TmpfsOptions{SizeBytes: 1024 * 1024, Mode: 0700},
			readOnly:             false,
			expectedSubstrings:   []string{"size=1m", "mode=700"},
			unexpectedSubstrings: []string{"ro"},
		},
		{
			opt:                  mount.TmpfsOptions{},
			readOnly:             true,
			expectedSubstrings:   []string{"ro"},
			unexpectedSubstrings: []string{},
		},
		{
			opt:                  mount.TmpfsOptions{Options: "exec"},
			readOnly:             true,
			expectedSubstrings:   []string{"ro", "exec"},
			unexpectedSubstrings: []string{"noexec"},
		},
		{
			opt: mount.TmpfsOptions{Options: "INVALID"},
			err: true,
		},
	}
	p := &linuxParser{}
	for _, c := range cases {
		data, err := p.ConvertTmpfsOptions(&c.opt, c.readOnly)
		if c.err {
			if err == nil {
				t.Fatalf("expected error for %+v, got nil", c.opt)
			}
			continue
		}
		if err != nil {
			t.Fatalf("could not convert %+v (readOnly: %v) to string: %v",
				c.opt, c.readOnly, err)
		}
		t.Logf("data=%q", data)
		for _, s := range c.expectedSubstrings {
			if !strings.Contains(data, s) {
				t.Fatalf("expected substring: %s, got %v (case=%+v)", s, data, c)
			}
		}
		for _, s := range c.unexpectedSubstrings {
			if strings.Contains(data, s) {
				t.Fatalf("unexpected substring: %s, got %v (case=%+v)", s, data, c)
			}
		}
	}
}

type mockFiProvider struct{}

func (mockFiProvider) fileInfo(path string) (exists, isDir bool, err error) {
	dirs := map[string]struct{}{
		`c:\`:                    {},
		`c:\windows\`:            {},
		`c:\windows`:             {},
		`c:\program files`:       {},
		`c:\Windows`:             {},
		`c:\Program Files (x86)`: {},
		`\\?\c:\windows\`:        {},
	}
	files := map[string]struct{}{
		`c:\windows\system32\ntdll.dll`: {},
	}
	if _, ok := dirs[path]; ok {
		return true, true, nil
	}
	if _, ok := files[path]; ok {
		return true, false, nil
	}
	return false, false, nil
}

// always returns the configured error
// this is used to test error handling
type mockFiProviderWithError struct{ err error }

func (m mockFiProviderWithError) fileInfo(path string) (bool, bool, error) {
	return false, false, m.err
}

func TestParseMountSpec(t *testing.T) {
	testDir, err := os.MkdirTemp("", "test-mount-config")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testDir)
	parser := NewParser()
	cases := []struct {
		input    mount.Mount
		expected MountPoint
	}{
		{
			input:    mount.Mount{Type: mount.TypeBind, Source: testDir, Target: testDestinationPath, ReadOnly: true},
			expected: MountPoint{Type: mount.TypeBind, Source: testDir, Destination: testDestinationPath, Propagation: parser.DefaultPropagationMode()},
		},
		{
			input:    mount.Mount{Type: mount.TypeBind, Source: testDir, Target: testDestinationPath},
			expected: MountPoint{Type: mount.TypeBind, Source: testDir, Destination: testDestinationPath, RW: true, Propagation: parser.DefaultPropagationMode()},
		},
		{
			input:    mount.Mount{Type: mount.TypeBind, Source: testDir + string(os.PathSeparator), Target: testDestinationPath, ReadOnly: true},
			expected: MountPoint{Type: mount.TypeBind, Source: testDir, Destination: testDestinationPath, Propagation: parser.DefaultPropagationMode()},
		},
		{
			input:    mount.Mount{Type: mount.TypeBind, Source: testDir, Target: testDestinationPath + string(os.PathSeparator), ReadOnly: true},
			expected: MountPoint{Type: mount.TypeBind, Source: testDir, Destination: testDestinationPath, Propagation: parser.DefaultPropagationMode()},
		},
		{
			input:    mount.Mount{Type: mount.TypeVolume, Target: testDestinationPath},
			expected: MountPoint{Type: mount.TypeVolume, Destination: testDestinationPath, RW: true, CopyData: parser.DefaultCopyMode()},
		},
		{
			input:    mount.Mount{Type: mount.TypeVolume, Target: testDestinationPath + string(os.PathSeparator)},
			expected: MountPoint{Type: mount.TypeVolume, Destination: testDestinationPath, RW: true, CopyData: parser.DefaultCopyMode()},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			mp, err := parser.ParseMountSpec(tc.input)
			assert.NilError(t, err)

			assert.Check(t, is.Equal(mp.Type, tc.expected.Type))
			assert.Check(t, is.Equal(mp.Destination, tc.expected.Destination))
			assert.Check(t, is.Equal(mp.Source, tc.expected.Source))
			assert.Check(t, is.Equal(mp.RW, tc.expected.RW))
			assert.Check(t, is.Equal(mp.Propagation, tc.expected.Propagation))
			assert.Check(t, is.Equal(mp.Driver, tc.expected.Driver))
			assert.Check(t, is.Equal(mp.CopyData, tc.expected.CopyData))
		})
	}
}
