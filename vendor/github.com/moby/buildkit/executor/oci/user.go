package oci

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/containerd/containerd/containers"
	containerdoci "github.com/containerd/containerd/oci"
	"github.com/containerd/continuity/fs"
	"github.com/opencontainers/runc/libcontainer/user"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func GetUser(root, username string) (uint32, uint32, []uint32, error) {
	var isDefault bool
	if username == "" {
		username = "0"
		isDefault = true
	}

	// fast path from uid/gid
	if uid, gid, err := ParseUIDGID(username); err == nil {
		return uid, gid, nil, nil
	}

	passwdFile, err := openUserFile(root, "/etc/passwd")
	if err == nil {
		defer passwdFile.Close()
	}
	groupFile, err := openUserFile(root, "/etc/group")
	if err == nil {
		defer groupFile.Close()
	}

	execUser, err := user.GetExecUser(username, nil, passwdFile, groupFile)
	if err != nil {
		if isDefault {
			return 0, 0, nil, nil
		}
		return 0, 0, nil, err
	}
	var sgids []uint32
	for _, g := range execUser.Sgids {
		sgids = append(sgids, uint32(g))
	}
	return uint32(execUser.Uid), uint32(execUser.Gid), sgids, nil
}

// ParseUIDGID takes the fast path to parse UID and GID if and only if they are both provided
func ParseUIDGID(str string) (uid uint32, gid uint32, err error) {
	if str == "" {
		return 0, 0, nil
	}
	parts := strings.SplitN(str, ":", 2)
	if len(parts) == 1 {
		return 0, 0, errors.New("groups ID is not provided")
	}
	if uid, err = parseUID(parts[0]); err != nil {
		return 0, 0, err
	}
	if gid, err = parseUID(parts[1]); err != nil {
		return 0, 0, err
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

// WithUIDGID allows the UID and GID for the Process to be set
// FIXME: This is a temporeray fix for the missing supplementary GIDs from containerd
// once the PR in containerd is merged we should remove this function.
func WithUIDGID(uid, gid uint32, sgids []uint32) containerdoci.SpecOpts {
	return func(_ context.Context, _ containerdoci.Client, _ *containers.Container, s *containerdoci.Spec) error {
		defer ensureAdditionalGids(s)
		setProcess(s)
		s.Process.User.UID = uid
		s.Process.User.GID = gid
		s.Process.User.AdditionalGids = sgids
		return nil
	}
}

// setProcess sets Process to empty if unset
// FIXME: Same on this one. Need to be removed after containerd fix merged
func setProcess(s *containerdoci.Spec) {
	if s.Process == nil {
		s.Process = &specs.Process{}
	}
}

// ensureAdditionalGids ensures that the primary GID is also included in the additional GID list.
// From https://github.com/containerd/containerd/blob/v1.7.0-beta.4/oci/spec_opts.go#L124-L133
func ensureAdditionalGids(s *containerdoci.Spec) {
	setProcess(s)
	for _, f := range s.Process.User.AdditionalGids {
		if f == s.Process.User.GID {
			return
		}
	}
	s.Process.User.AdditionalGids = append([]uint32{s.Process.User.GID}, s.Process.User.AdditionalGids...)
}
