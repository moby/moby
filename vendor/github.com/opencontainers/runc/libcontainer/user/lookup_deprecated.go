package user

import (
	"io"

	"github.com/moby/sys/user"
)

// LookupUser looks up a user by their username in /etc/passwd. If the user
// cannot be found (or there is no /etc/passwd file on the filesystem), then
// LookupUser returns an error.
func LookupUser(username string) (user.User, error) {
	return user.LookupUser(username)
}

// LookupUid looks up a user by their user id in /etc/passwd. If the user cannot
// be found (or there is no /etc/passwd file on the filesystem), then LookupId
// returns an error.
func LookupUid(uid int) (user.User, error) { //nolint:revive // ignore var-naming: func LookupUid should be LookupUID
	return user.LookupUid(uid)
}

// LookupGroup looks up a group by its name in /etc/group. If the group cannot
// be found (or there is no /etc/group file on the filesystem), then LookupGroup
// returns an error.
func LookupGroup(groupname string) (user.Group, error) {
	return user.LookupGroup(groupname)
}

// LookupGid looks up a group by its group id in /etc/group. If the group cannot
// be found (or there is no /etc/group file on the filesystem), then LookupGid
// returns an error.
func LookupGid(gid int) (user.Group, error) {
	return user.LookupGid(gid)
}

func GetPasswdPath() (string, error) {
	return user.GetPasswdPath()
}

func GetPasswd() (io.ReadCloser, error) {
	return user.GetPasswd()
}

func GetGroupPath() (string, error) {
	return user.GetGroupPath()
}

func GetGroup() (io.ReadCloser, error) {
	return user.GetGroup()
}

// CurrentUser looks up the current user by their user id in /etc/passwd. If the
// user cannot be found (or there is no /etc/passwd file on the filesystem),
// then CurrentUser returns an error.
func CurrentUser() (user.User, error) {
	return user.CurrentUser()
}

// CurrentGroup looks up the current user's group by their primary group id's
// entry in /etc/passwd. If the group cannot be found (or there is no
// /etc/group file on the filesystem), then CurrentGroup returns an error.
func CurrentGroup() (user.Group, error) {
	return user.CurrentGroup()
}

func CurrentUserSubUIDs() ([]user.SubID, error) {
	return user.CurrentUserSubUIDs()
}

func CurrentUserSubGIDs() ([]user.SubID, error) {
	return user.CurrentUserSubGIDs()
}

func CurrentProcessUIDMap() ([]user.IDMap, error) {
	return user.CurrentProcessUIDMap()
}

func CurrentProcessGIDMap() ([]user.IDMap, error) {
	return user.CurrentProcessGIDMap()
}
