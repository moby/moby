package convert

import (
	"testing"

	"github.com/docker/docker/api/types/mount"
	composetypes "github.com/docker/docker/cli/compose/types"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestIsReadOnly(t *testing.T) {
	assert.Equal(t, isReadOnly([]string{"foo", "bar", "ro"}), true)
	assert.Equal(t, isReadOnly([]string{"ro"}), true)
	assert.Equal(t, isReadOnly([]string{}), false)
	assert.Equal(t, isReadOnly([]string{"foo", "rw"}), false)
	assert.Equal(t, isReadOnly([]string{"foo"}), false)
}

func TestIsNoCopy(t *testing.T) {
	assert.Equal(t, isNoCopy([]string{"foo", "bar", "nocopy"}), true)
	assert.Equal(t, isNoCopy([]string{"nocopy"}), true)
	assert.Equal(t, isNoCopy([]string{}), false)
	assert.Equal(t, isNoCopy([]string{"foo", "rw"}), false)
}

func TestGetBindOptions(t *testing.T) {
	opts := getBindOptions([]string{"slave"})
	expected := mount.BindOptions{Propagation: mount.PropagationSlave}
	assert.Equal(t, *opts, expected)
}

func TestGetBindOptionsNone(t *testing.T) {
	opts := getBindOptions([]string{"ro"})
	assert.Equal(t, opts, (*mount.BindOptions)(nil))
}

func TestConvertVolumeToMountNamedVolume(t *testing.T) {
	stackVolumes := volumes{
		"normal": composetypes.VolumeConfig{
			Driver: "glusterfs",
			DriverOpts: map[string]string{
				"opt": "value",
			},
			Labels: map[string]string{
				"something": "labeled",
			},
		},
	}
	namespace := NewNamespace("foo")
	expected := mount.Mount{
		Type:     mount.TypeVolume,
		Source:   "foo_normal",
		Target:   "/foo",
		ReadOnly: true,
		VolumeOptions: &mount.VolumeOptions{
			Labels: map[string]string{
				LabelNamespace: "foo",
				"something":    "labeled",
			},
			DriverConfig: &mount.Driver{
				Name: "glusterfs",
				Options: map[string]string{
					"opt": "value",
				},
			},
		},
	}
	mount, err := convertVolumeToMount("normal:/foo:ro", stackVolumes, namespace)
	assert.NilError(t, err)
	assert.DeepEqual(t, mount, expected)
}

func TestConvertVolumeToMountNamedVolumeExternal(t *testing.T) {
	stackVolumes := volumes{
		"outside": composetypes.VolumeConfig{
			External: composetypes.External{
				External: true,
				Name:     "special",
			},
		},
	}
	namespace := NewNamespace("foo")
	expected := mount.Mount{
		Type:   mount.TypeVolume,
		Source: "special",
		Target: "/foo",
	}
	mount, err := convertVolumeToMount("outside:/foo", stackVolumes, namespace)
	assert.NilError(t, err)
	assert.DeepEqual(t, mount, expected)
}

func TestConvertVolumeToMountBind(t *testing.T) {
	stackVolumes := volumes{}
	namespace := NewNamespace("foo")
	expected := mount.Mount{
		Type:        mount.TypeBind,
		Source:      "/bar",
		Target:      "/foo",
		ReadOnly:    true,
		BindOptions: &mount.BindOptions{Propagation: mount.PropagationShared},
	}
	mount, err := convertVolumeToMount("/bar:/foo:ro,shared", stackVolumes, namespace)
	assert.NilError(t, err)
	assert.DeepEqual(t, mount, expected)
}

func TestConvertVolumeToMountVolumeDoesNotExist(t *testing.T) {
	namespace := NewNamespace("foo")
	_, err := convertVolumeToMount("unknown:/foo:ro", volumes{}, namespace)
	assert.Error(t, err, "undefined volume: unknown")
}

func TestConvertVolumeToMountAnonymousVolume(t *testing.T) {
	stackVolumes := map[string]composetypes.VolumeConfig{}
	namespace := NewNamespace("foo")
	expected := mount.Mount{
		Type:   mount.TypeVolume,
		Target: "/foo/bar",
	}
	mnt, err := convertVolumeToMount("/foo/bar", stackVolumes, namespace)
	assert.NilError(t, err)
	assert.DeepEqual(t, mnt, expected)
}

func TestConvertVolumeToMountInvalidFormat(t *testing.T) {
	namespace := NewNamespace("foo")
	invalids := []string{"::", "::cc", ":bb:", "aa::", "aa::cc", "aa:bb:", " : : ", " : :cc", " :bb: ", "aa: : ", "aa: :cc", "aa:bb: "}
	for _, vol := range invalids {
		_, err := convertVolumeToMount(vol, map[string]composetypes.VolumeConfig{}, namespace)
		assert.Error(t, err, "invalid volume: "+vol)
	}
}
