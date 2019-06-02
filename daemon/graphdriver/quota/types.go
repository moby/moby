// +build linux

package quota // import "github.com/docker/docker/daemon/graphdriver/quota"

// Quota limit params - currently we only control blocks hard limit
type Quota struct {
	Size uint64
}

// Control - Context to be used by storage driver (e.g. overlay)
// who wants to apply project quotas to container dirs
type Control struct {
	backingFsBlockDev string
	nextProjectID     uint32
	quotas            map[string]uint32
}
