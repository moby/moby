//go:build !windows
// +build !windows

package listeners // import "github.com/docker/docker/daemon/listeners"

import (
	"fmt"
	"strconv"

	"github.com/docker/docker/pkg/idtools"
)

const defaultSocketGroup = "docker"

func lookupGID(name string) (int, error) {
	group, err := idtools.LookupGroup(name)
	if err == nil {
		return group.Gid, nil
	}
	gid, err := strconv.Atoi(name)
	if err == nil {
		return gid, nil
	}
	return -1, fmt.Errorf("group %s not found", name)
}
