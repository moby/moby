package loader

import (
	"testing"

	"github.com/docker/docker/cli/compose/types"
	"github.com/docker/docker/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestParseVolumeAnonymousVolume(t *testing.T) {
	for _, path := range []string{"/path", "/path/foo"} {
		volume, err := parseVolume(path)
		expected := types.ServiceVolumeConfig{Type: "volume", Target: path}
		assert.NoError(t, err)
		assert.Equal(t, expected, volume)
	}
}

func TestParseVolumeAnonymousVolumeWindows(t *testing.T) {
	for _, path := range []string{"C:\\path", "Z:\\path\\foo"} {
		volume, err := parseVolume(path)
		expected := types.ServiceVolumeConfig{Type: "volume", Target: path}
		assert.NoError(t, err)
		assert.Equal(t, expected, volume)
	}
}

func TestParseVolumeTooManyColons(t *testing.T) {
	_, err := parseVolume("/foo:/foo:ro:foo")
	assert.EqualError(t, err, "invalid spec: /foo:/foo:ro:foo: too many colons")
}

func TestParseVolumeShortVolumes(t *testing.T) {
	for _, path := range []string{".", "/a"} {
		volume, err := parseVolume(path)
		expected := types.ServiceVolumeConfig{Type: "volume", Target: path}
		assert.NoError(t, err)
		assert.Equal(t, expected, volume)
	}
}

func TestParseVolumeMissingSource(t *testing.T) {
	for _, spec := range []string{":foo", "/foo::ro"} {
		_, err := parseVolume(spec)
		testutil.ErrorContains(t, err, "empty section between colons")
	}
}

func TestParseVolumeBindMount(t *testing.T) {
	for _, path := range []string{"./foo", "~/thing", "../other", "/foo", "/home/user"} {
		volume, err := parseVolume(path + ":/target")
		expected := types.ServiceVolumeConfig{
			Type:   "bind",
			Source: path,
			Target: "/target",
		}
		assert.NoError(t, err)
		assert.Equal(t, expected, volume)
	}
}

func TestParseVolumeRelativeBindMountWindows(t *testing.T) {
	for _, path := range []string{
		"./foo",
		"~/thing",
		"../other",
		"D:\\path", "/home/user",
	} {
		volume, err := parseVolume(path + ":d:\\target")
		expected := types.ServiceVolumeConfig{
			Type:   "bind",
			Source: path,
			Target: "d:\\target",
		}
		assert.NoError(t, err)
		assert.Equal(t, expected, volume)
	}
}

func TestParseVolumeWithBindOptions(t *testing.T) {
	volume, err := parseVolume("/source:/target:slave")
	expected := types.ServiceVolumeConfig{
		Type:   "bind",
		Source: "/source",
		Target: "/target",
		Bind:   &types.ServiceVolumeBind{Propagation: "slave"},
	}
	assert.NoError(t, err)
	assert.Equal(t, expected, volume)
}

func TestParseVolumeWithBindOptionsWindows(t *testing.T) {
	volume, err := parseVolume("C:\\source\\foo:D:\\target:ro,rprivate")
	expected := types.ServiceVolumeConfig{
		Type:     "bind",
		Source:   "C:\\source\\foo",
		Target:   "D:\\target",
		ReadOnly: true,
		Bind:     &types.ServiceVolumeBind{Propagation: "rprivate"},
	}
	assert.NoError(t, err)
	assert.Equal(t, expected, volume)
}

func TestParseVolumeWithInvalidVolumeOptions(t *testing.T) {
	_, err := parseVolume("name:/target:bogus")
	assert.EqualError(t, err, "invalid spec: name:/target:bogus: unknown option: bogus")
}

func TestParseVolumeWithVolumeOptions(t *testing.T) {
	volume, err := parseVolume("name:/target:nocopy")
	expected := types.ServiceVolumeConfig{
		Type:   "volume",
		Source: "name",
		Target: "/target",
		Volume: &types.ServiceVolumeVolume{NoCopy: true},
	}
	assert.NoError(t, err)
	assert.Equal(t, expected, volume)
}

func TestParseVolumeWithReadOnly(t *testing.T) {
	for _, path := range []string{"./foo", "/home/user"} {
		volume, err := parseVolume(path + ":/target:ro")
		expected := types.ServiceVolumeConfig{
			Type:     "bind",
			Source:   path,
			Target:   "/target",
			ReadOnly: true,
		}
		assert.NoError(t, err)
		assert.Equal(t, expected, volume)
	}
}

func TestParseVolumeWithRW(t *testing.T) {
	for _, path := range []string{"./foo", "/home/user"} {
		volume, err := parseVolume(path + ":/target:rw")
		expected := types.ServiceVolumeConfig{
			Type:     "bind",
			Source:   path,
			Target:   "/target",
			ReadOnly: false,
		}
		assert.NoError(t, err)
		assert.Equal(t, expected, volume)
	}
}
