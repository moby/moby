package graphdriver // import "github.com/docker/docker/daemon/graphdriver"

// List of drivers that should be used in an order
var priority = "overlay2,fuse-overlayfs,btrfs,zfs,vfs"
