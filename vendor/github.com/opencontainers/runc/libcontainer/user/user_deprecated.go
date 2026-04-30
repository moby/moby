// Package user is an alias for [github.com/moby/sys/user].
//
// Deprecated: use [github.com/moby/sys/user].
package user

import (
	"io"

	"github.com/moby/sys/user"
)

var (
	// ErrNoPasswdEntries is returned if no matching entries were found in /etc/group.
	ErrNoPasswdEntries = user.ErrNoPasswdEntries
	// ErrNoGroupEntries is returned if no matching entries were found in /etc/passwd.
	ErrNoGroupEntries = user.ErrNoGroupEntries
	// ErrRange is returned if a UID or GID is outside of the valid range.
	ErrRange = user.ErrRange
)

type (
	User = user.User

	Group = user.Group

	// SubID represents an entry in /etc/sub{u,g}id.
	SubID = user.SubID

	// IDMap represents an entry in /proc/PID/{u,g}id_map.
	IDMap = user.IDMap

	ExecUser = user.ExecUser
)

func ParsePasswdFile(path string) ([]user.User, error) {
	return user.ParsePasswdFile(path)
}

func ParsePasswd(passwd io.Reader) ([]user.User, error) {
	return user.ParsePasswd(passwd)
}

func ParsePasswdFileFilter(path string, filter func(user.User) bool) ([]user.User, error) {
	return user.ParsePasswdFileFilter(path, filter)
}

func ParsePasswdFilter(r io.Reader, filter func(user.User) bool) ([]user.User, error) {
	return user.ParsePasswdFilter(r, filter)
}

func ParseGroupFile(path string) ([]user.Group, error) {
	return user.ParseGroupFile(path)
}

func ParseGroup(group io.Reader) ([]user.Group, error) {
	return user.ParseGroup(group)
}

func ParseGroupFileFilter(path string, filter func(user.Group) bool) ([]user.Group, error) {
	return user.ParseGroupFileFilter(path, filter)
}

func ParseGroupFilter(r io.Reader, filter func(user.Group) bool) ([]user.Group, error) {
	return user.ParseGroupFilter(r, filter)
}

// GetExecUserPath is a wrapper for GetExecUser. It reads data from each of the
// given file paths and uses that data as the arguments to GetExecUser. If the
// files cannot be opened for any reason, the error is ignored and a nil
// io.Reader is passed instead.
func GetExecUserPath(userSpec string, defaults *user.ExecUser, passwdPath, groupPath string) (*user.ExecUser, error) {
	return user.GetExecUserPath(userSpec, defaults, passwdPath, groupPath)
}

// GetExecUser parses a user specification string (using the passwd and group
// readers as sources for /etc/passwd and /etc/group data, respectively). In
// the case of blank fields or missing data from the sources, the values in
// defaults is used.
//
// GetExecUser will return an error if a user or group literal could not be
// found in any entry in passwd and group respectively.
//
// Examples of valid user specifications are:
//   - ""
//   - "user"
//   - "uid"
//   - "user:group"
//   - "uid:gid
//   - "user:gid"
//   - "uid:group"
//
// It should be noted that if you specify a numeric user or group id, they will
// not be evaluated as usernames (only the metadata will be filled). So attempting
// to parse a user with user.Name = "1337" will produce the user with a UID of
// 1337.
func GetExecUser(userSpec string, defaults *user.ExecUser, passwd, group io.Reader) (*user.ExecUser, error) {
	return user.GetExecUser(userSpec, defaults, passwd, group)
}

// GetAdditionalGroups looks up a list of groups by name or group id
// against the given /etc/group formatted data. If a group name cannot
// be found, an error will be returned. If a group id cannot be found,
// or the given group data is nil, the id will be returned as-is
// provided it is in the legal range.
func GetAdditionalGroups(additionalGroups []string, group io.Reader) ([]int, error) {
	return user.GetAdditionalGroups(additionalGroups, group)
}

// GetAdditionalGroupsPath is a wrapper around GetAdditionalGroups
// that opens the groupPath given and gives it as an argument to
// GetAdditionalGroups.
func GetAdditionalGroupsPath(additionalGroups []string, groupPath string) ([]int, error) {
	return user.GetAdditionalGroupsPath(additionalGroups, groupPath)
}

func ParseSubIDFile(path string) ([]user.SubID, error) {
	return user.ParseSubIDFile(path)
}

func ParseSubID(subid io.Reader) ([]user.SubID, error) {
	return user.ParseSubID(subid)
}

func ParseSubIDFileFilter(path string, filter func(user.SubID) bool) ([]user.SubID, error) {
	return user.ParseSubIDFileFilter(path, filter)
}

func ParseSubIDFilter(r io.Reader, filter func(user.SubID) bool) ([]user.SubID, error) {
	return user.ParseSubIDFilter(r, filter)
}

func ParseIDMapFile(path string) ([]user.IDMap, error) {
	return user.ParseIDMapFile(path)
}

func ParseIDMap(r io.Reader) ([]user.IDMap, error) {
	return user.ParseIDMap(r)
}

func ParseIDMapFileFilter(path string, filter func(user.IDMap) bool) ([]user.IDMap, error) {
	return user.ParseIDMapFileFilter(path, filter)
}

func ParseIDMapFilter(r io.Reader, filter func(user.IDMap) bool) ([]user.IDMap, error) {
	return user.ParseIDMapFilter(r, filter)
}
