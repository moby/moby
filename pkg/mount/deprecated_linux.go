package mount // import "github.com/moby/moby/pkg/mount"

import (
	sysmount "github.com/moby/sys/mount"
)

// Deprecated: use github.com/moby/sys/mount instead.
//nolint:golint
var (
	MakeMount       = sysmount.MakeMount
	MakeShared      = sysmount.MakeShared
	MakeRShared     = sysmount.MakeRShared
	MakePrivate     = sysmount.MakePrivate
	MakeRPrivate    = sysmount.MakeRPrivate
	MakeSlave       = sysmount.MakeSlave
	MakeRSlave      = sysmount.MakeRSlave
	MakeUnbindable  = sysmount.MakeUnbindable
	MakeRUnbindable = sysmount.MakeRUnbindable
)
