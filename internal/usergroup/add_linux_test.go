package usergroup

import (
	"os"
	"os/exec"
	"os/user"
	"syscall"
	"testing"

	mobyuser "github.com/moby/sys/user"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

const (
	tempUser = "tempuser"
)

func TestNewIDMappings(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	_, _, err := AddNamespaceRangesUser(tempUser)
	assert.Check(t, err)
	defer delUser(t, tempUser)

	tempUser, err := user.Lookup(tempUser)
	assert.Check(t, err)

	idMapping, err := LoadIdentityMapping(tempUser.Username)
	assert.Check(t, err)

	rootUID, rootGID := idMapping.RootPair()

	dirName, err := os.MkdirTemp("", "mkdirall")
	assert.Check(t, err, "Couldn't create temp directory")
	defer os.RemoveAll(dirName)

	err = mobyuser.MkdirAllAndChown(dirName, 0o700, rootUID, rootGID)
	assert.Check(t, err, "Couldn't change ownership of file path. Got error")
	cmd := exec.Command("ls", "-la", dirName)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(rootUID), Gid: uint32(rootGID)},
	}
	out, err := cmd.CombinedOutput()
	assert.Check(t, err, "Unable to access %s directory with user UID:%d and GID:%d:\n%s", dirName, rootUID, rootGID, string(out))
}

func TestLookupUserAndGroup(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	uid, gid, err := AddNamespaceRangesUser(tempUser)
	assert.Check(t, err)
	defer delUser(t, tempUser)

	fetchedUser, err := LookupUser(tempUser)
	assert.Check(t, err)

	fetchedUserByID, err := LookupUID(uid)
	assert.Check(t, err)
	assert.Check(t, is.DeepEqual(fetchedUserByID, fetchedUser))

	fetchedGroup, err := LookupGroup(tempUser)
	assert.Check(t, err)

	fetchedGroupByID, err := LookupGID(gid)
	assert.Check(t, err)
	assert.Check(t, is.DeepEqual(fetchedGroupByID, fetchedGroup))
}

func delUser(t *testing.T, name string) {
	out, err := exec.Command("userdel", name).CombinedOutput()
	assert.Check(t, err, out)
}
