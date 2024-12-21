package idtools // import "github.com/docker/docker/pkg/idtools"

import (
	"os"
)

const (
	SeTakeOwnershipPrivilege = "SeTakeOwnershipPrivilege"
)

const (
	ContainerAdministratorSidString = "S-1-5-93-2-1"
	ContainerUserSidString          = "S-1-5-93-2-2"
)

// This is currently a wrapper around [os.MkdirAll] since currently
// permissions aren't set through this path, the identity isn't utilized.
// Ownership is handled elsewhere, but in the future could be support here
// too.
func mkdirAs(path string, _ os.FileMode, _ Identity, _, _ bool) error {
	return os.MkdirAll(path, 0)
}
