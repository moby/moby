package usergroup

import (
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/sys/user"
)

const (
	subuidFileName = "/etc/subuid"
	subgidFileName = "/etc/subgid"
)

func createIDMap(subidRanges []user.SubID) []idtools.IDMap {
	idMap := []idtools.IDMap{}

	containerID := 0
	for _, idrange := range subidRanges {
		idMap = append(idMap, idtools.IDMap{
			ContainerID: containerID,
			HostID:      int(idrange.SubID),
			Size:        int(idrange.Count),
		})
		containerID = containerID + int(idrange.Count)
	}
	return idMap
}

func parseSubuid(username string) ([]user.SubID, error) {
	return user.ParseSubIDFileFilter(subuidFileName, func(sid user.SubID) bool {
		return sid.Name == username
	})
}

func parseSubgid(username string) ([]user.SubID, error) {
	return user.ParseSubIDFileFilter(subgidFileName, func(sid user.SubID) bool {
		return sid.Name == username
	})
}
