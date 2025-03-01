//go:build !windows

package idtools

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestGetRootUIDGID(t *testing.T) {
	uidMap := []IDMap{
		{
			ContainerID: 0,
			HostID:      os.Getuid(),
			Size:        1,
		},
	}
	gidMap := []IDMap{
		{
			ContainerID: 0,
			HostID:      os.Getgid(),
			Size:        1,
		},
	}

	uid, gid, err := getRootUIDGID(uidMap, gidMap)
	assert.Check(t, err)
	assert.Check(t, is.Equal(os.Geteuid(), uid))
	assert.Check(t, is.Equal(os.Getegid(), gid))

	uidMapError := []IDMap{
		{
			ContainerID: 1,
			HostID:      os.Getuid(),
			Size:        1,
		},
	}
	_, _, err = getRootUIDGID(uidMapError, gidMap)
	assert.Check(t, is.Error(err, "Container ID 0 cannot be mapped to a host ID"))
}

func TestToContainer(t *testing.T) {
	uidMap := []IDMap{
		{
			ContainerID: 2,
			HostID:      2,
			Size:        1,
		},
	}

	containerID, err := toContainer(2, uidMap)
	assert.Check(t, err)
	assert.Check(t, is.Equal(uidMap[0].ContainerID, containerID))
}
