package oci

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/containerd/continuity/fs"
	"github.com/opencontainers/runc/libcontainer/user"
)

func GetUser(ctx context.Context, root, username string) (uint32, uint32, error) {
	// fast path from uid/gid
	if uid, gid, err := ParseUser(username); err == nil {
		return uid, gid, nil
	}

	passwdPath, err := user.GetPasswdPath()
	if err != nil {
		return 0, 0, err
	}
	groupPath, err := user.GetGroupPath()
	if err != nil {
		return 0, 0, err
	}
	passwdFile, err := openUserFile(root, passwdPath)
	if err == nil {
		defer passwdFile.Close()
	}
	groupFile, err := openUserFile(root, groupPath)
	if err == nil {
		defer groupFile.Close()
	}

	execUser, err := user.GetExecUser(username, nil, passwdFile, groupFile)
	if err != nil {
		return 0, 0, err
	}

	return uint32(execUser.Uid), uint32(execUser.Gid), nil
}

func ParseUser(str string) (uid uint32, gid uint32, err error) {
	if str == "" {
		return 0, 0, nil
	}
	parts := strings.SplitN(str, ":", 2)
	for i, v := range parts {
		switch i {
		case 0:
			uid, err = parseUID(v)
			if err != nil {
				return 0, 0, err
			}
			if len(parts) == 1 {
				gid = uid
			}
		case 1:
			gid, err = parseUID(v)
			if err != nil {
				return 0, 0, err
			}
		}
	}
	return
}

func openUserFile(root, p string) (*os.File, error) {
	p, err := fs.RootPath(root, p)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

func parseUID(str string) (uint32, error) {
	if str == "root" {
		return 0, nil
	}
	uid, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(uid), nil
}
