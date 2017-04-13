package convert

import (
	"testing"

	"github.com/docker/docker/api/types/mount"
	composetypes "github.com/docker/docker/cli/compose/types"
	"github.com/stretchr/testify/assert"
)

func TestConvertVolumeToMountAnonymousVolume(t *testing.T) {
	config := composetypes.ServiceVolumeConfig{
		Type:   "volume",
		Target: "/foo/bar",
	}
	expected := mount.Mount{
		Type:   mount.TypeVolume,
		Target: "/foo/bar",
	}
	mount, err := convertVolumeToMount(config, volumes{}, NewNamespace("foo"))
	assert.NoError(t, err)
	assert.Equal(t, expected, mount)
}

func TestConvertVolumeToMountConflictingOptionsBind(t *testing.T) {
	namespace := NewNamespace("foo")

	config := composetypes.ServiceVolumeConfig{
		Type:   "volume",
		Source: "foo",
		Target: "/target",
		Bind: &composetypes.ServiceVolumeBind{
			Propagation: "slave",
		},
	}
	_, err := convertVolumeToMount(config, volumes{}, namespace)
	assert.EqualError(t, err, "bind options are incompatible with type volume")
}

func TestConvertVolumeToMountConflictingOptionsVolume(t *testing.T) {
	namespace := NewNamespace("foo")

	config := composetypes.ServiceVolumeConfig{
		Type:   "bind",
		Source: "/foo",
		Target: "/target",
		Volume: &composetypes.ServiceVolumeVolume{
			NoCopy: true,
		},
	}
	_, err := convertVolumeToMount(config, volumes{}, namespace)
	assert.EqualError(t, err, "volume options are incompatible with type bind")
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
			NoCopy: true,
		},
	}
	config := composetypes.ServiceVolumeConfig{
		Type:     "volume",
		Source:   "normal",
		Target:   "/foo",
		ReadOnly: true,
		Volume: &composetypes.ServiceVolumeVolume{
			NoCopy: true,
		},
	}
	mount, err := convertVolumeToMount(config, stackVolumes, namespace)
	assert.NoError(t, err)
	assert.Equal(t, expected, mount)
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
		VolumeOptions: &mount.VolumeOptions{
			NoCopy: false,
		},
	}
	config := composetypes.ServiceVolumeConfig{
		Type:   "volume",
		Source: "outside",
		Target: "/foo",
	}
	mount, err := convertVolumeToMount(config, stackVolumes, namespace)
	assert.NoError(t, err)
	assert.Equal(t, expected, mount)
}

func TestConvertVolumeToMountNamedVolumeExternalNoCopy(t *testing.T) {
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
		VolumeOptions: &mount.VolumeOptions{
			NoCopy: true,
		},
	}
	config := composetypes.ServiceVolumeConfig{
		Type:   "volume",
		Source: "outside",
		Target: "/foo",
		Volume: &composetypes.ServiceVolumeVolume{
			NoCopy: true,
		},
	}
	mount, err := convertVolumeToMount(config, stackVolumes, namespace)
	assert.NoError(t, err)
	assert.Equal(t, expected, mount)
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
	config := composetypes.ServiceVolumeConfig{
		Type:     "bind",
		Source:   "/bar",
		Target:   "/foo",
		ReadOnly: true,
		Bind:     &composetypes.ServiceVolumeBind{Propagation: "shared"},
	}
	mount, err := convertVolumeToMount(config, stackVolumes, namespace)
	assert.NoError(t, err)
	assert.Equal(t, expected, mount)
}

func TestConvertVolumeToMountVolumeDoesNotExist(t *testing.T) {
	namespace := NewNamespace("foo")
	config := composetypes.ServiceVolumeConfig{
		Type:     "volume",
		Source:   "unknown",
		Target:   "/foo",
		ReadOnly: true,
	}
	_, err := convertVolumeToMount(config, volumes{}, namespace)
	assert.EqualError(t, err, "undefined volume \"unknown\"")
}
