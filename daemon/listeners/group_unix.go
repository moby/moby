//go:build !windows
// +build !windows

package listeners // import "github.com/docker/docker/daemon/listeners"

import (
	"fmt"
	"os/user"
	"strconv"
)

const defaultSocketGroup = "docker"

func lookupGID(name string) (int, error) {
	group, err := user.LookupGroup(name)
	if err != nil {
		gid, err := strconv.Atoi(name)
		if err != nil {
			return -1, fmt.Errorf("group %s not found", name)
		}
		return gid, nil
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		// group.Gid is documented to always be numeric on POSIX
		// systems.
		panic(fmt.Errorf("gid is not numeric: %w", err))
	}
	return gid, nil
}
