package windows

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"syscall"

	"github.com/containerd/containerd/mount"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/snapshot"
	"github.com/pkg/errors"
)

func ResolveUsernameToSID(ctx context.Context, exec executor.Executor, rootMount []mount.Mount, userName string) (idtools.Identity, error) {
	// This is a shortcut in case the user is one of the builtin users that should exist
	// in any WCOW container. While these users do exist in containers, they don't exist on the
	// host. We check them before trying to look them up using LookupSID().
	if strings.EqualFold(userName, "ContainerAdministrator") || userName == "" {
		return idtools.Identity{SID: idtools.ContainerAdministratorSidString}, nil
	} else if strings.EqualFold(userName, "ContainerUser") {
		return idtools.Identity{SID: idtools.ContainerUserSidString}, nil
	}

	// We might have a SID set as username. There is no guarantee that this SID will exist
	// inside the container, even if we can successfully parse it. If a SID was used, we trust
	// that the user has made sure it does map to an identity inside the container.
	if strings.HasPrefix(strings.ToLower(userName), "s-") {
		if _, err := syscall.StringToSid(userName); err == nil {
			return idtools.Identity{SID: userName}, nil
		}
	}

	// We test for well known accounts that should exist inside any system. This has the potential
	// to fail if the usernames/group names differ in the container, as is the case on internationalized
	// versions where the builtin account and group names may have names in the local language.
	// If the user specified an internationalized version of the account name, but the host is in English,
	// this lookup will most likely fail and we will fall back to running get-account-info inside the container.
	// This should however catch most of the cases when well known accounts/groups are used.
	sid, _, accountType, err := syscall.LookupSID("", userName)
	if err == nil {
		if accountType == syscall.SidTypeAlias || accountType == syscall.SidTypeWellKnownGroup {
			sidAsString, err := sid.String()
			if err == nil {
				return idtools.Identity{SID: sidAsString}, nil
			}
		}
	}

	// Last resort.
	// The equivalent in Windows of /etc/passwd and /etc/group is a registry hive called SAM which can be found
	// on any windows system in: C:\Windows\System32\config\SAM.
	//
	// This hive holds all user information on a particular system, including the SID of the user we care
	// about. The bad news is that the data structures in this hive are completely undocumented and there
	// is no API we can call to load the security info inside an offline SAM hive. We can load it as a
	// registry hive, but parsing the data structures it holds is not documented. It's not impossible to do,
	// but in the absence of a supported API to do this for us, we risk that sometime in the future our parser
	// will break.
	//
	// That being said, we have no choice but to execute a command inside the rootMount and attempt to get the
	// SID of the user we care about using officially supported Windows APIs. This obviously adds some overhead.
	//
	// TODO(gsamfira): Should we use a snapshot of the rootMount?
	ident, err := GetUserIdentFromContainer(ctx, exec, rootMount, userName)
	if err != nil {
		return idtools.Identity{}, errors.Wrap(err, "getting account SID from container")
	}
	return ident, nil
}

func GetUserIdentFromContainer(ctx context.Context, exec executor.Executor, rootMounts []mount.Mount, userName string) (idtools.Identity, error) {
	var ident idtools.Identity

	if len(rootMounts) > 1 {
		return ident, errors.Errorf("unexpected number of root mounts: %d", len(rootMounts))
	}

	stdout := &bytesReadWriteCloser{
		bw: &bytes.Buffer{},
	}
	stderr := &bytesReadWriteCloser{
		bw: &bytes.Buffer{},
	}

	defer stdout.Close()
	defer stderr.Close()

	procInfo := executor.ProcessInfo{
		Meta: executor.Meta{
			Args: []string{"get-user-info", userName},
			User: "ContainerAdministrator",
			Cwd:  "/",
		},
		Stdin:  nil,
		Stdout: stdout,
		Stderr: stderr,
	}

	if _, err := exec.Run(ctx, "", newStubMountable(rootMounts), nil, procInfo, nil); err != nil {
		return ident, errors.Wrap(err, "executing command")
	}

	data := stdout.bw.Bytes()
	if err := json.Unmarshal(data, &ident); err != nil {
		return ident, errors.Wrap(err, "reading user info")
	}

	return ident, nil
}

type bytesReadWriteCloser struct {
	bw *bytes.Buffer
}

func (b *bytesReadWriteCloser) Write(p []byte) (int, error) {
	if b.bw == nil {
		return 0, errors.Errorf("invalid bytes buffer")
	}
	return b.bw.Write(p)
}

func (b *bytesReadWriteCloser) Close() error {
	if b.bw == nil {
		return nil
	}
	b.bw.Reset()
	return nil
}

type snapshotMountable struct {
	m []mount.Mount
}

func (m *snapshotMountable) Mount() ([]mount.Mount, func() error, error) {
	cleanup := func() error { return nil }
	return m.m, cleanup, nil
}
func (m *snapshotMountable) IdentityMapping() *idtools.IdentityMapping {
	return nil
}

type executorMountable struct {
	m snapshot.Mountable
}

func (m *executorMountable) Mount(ctx context.Context, readonly bool) (snapshot.Mountable, error) {
	return m.m, nil
}

func newStubMountable(m []mount.Mount) executor.Mount {
	return executor.Mount{Src: &executorMountable{m: &snapshotMountable{m: m}}}
}
