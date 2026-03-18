package usergroup

import (
	"github.com/moby/sys/user"
)

const (
	subuidFileName = "/etc/subuid"
	subgidFileName = "/etc/subgid"
)

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
