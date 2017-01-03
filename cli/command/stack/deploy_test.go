package stack

import (
	"testing"

	composetypes "github.com/aanand/compose-file/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestConvertVolumeToMountAnonymousVolume(t *testing.T) {
	stackVolumes := map[string]composetypes.VolumeConfig{}
	namespace := namespace{name: "foo"}
	expected := mount.Mount{
		Type:   mount.TypeVolume,
		Target: "/foo/bar",
	}
	mnt, err := convertVolumeToMount("/foo/bar", stackVolumes, namespace)
	assert.NilError(t, err)
	assert.DeepEqual(t, mnt, expected)
}

func TestConvertVolumeToMountInvalidFormat(t *testing.T) {
	namespace := namespace{name: "foo"}
	invalids := []string{"::", "::cc", ":bb:", "aa::", "aa::cc", "aa:bb:", " : : ", " : :cc", " :bb: ", "aa: : ", "aa: :cc", "aa:bb: "}
	for _, vol := range invalids {
		_, err := convertVolumeToMount(vol, map[string]composetypes.VolumeConfig{}, namespace)
		assert.Error(t, err, "invalid volume: "+vol)
	}
}
