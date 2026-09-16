//go:build windows

package mounts

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/v2/core/mount"
	"github.com/moby/buildkit/identity"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// Mount writes the secret to a temp file and returns it as a single-file,
// read-only bind mount. Windows has no tmpfs, so it is stored on disk; UID/GID/
// mode are unsupported and ignored.
func (sm *secretMountInstance) Mount() ([]mount.Mount, func() error, error) {
	dir, err := os.MkdirTemp("", "buildkit-secrets")
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create temp dir")
	}
	cleanup := func() error {
		return os.RemoveAll(dir)
	}
	sm.root = dir

	// Restrict the temp dir before writing the secret. The inheritable ACL
	// ensures the secret file is born restricted (no window where it carries
	// the broad inherited %TEMP% permissions) and removes any reliance on the
	// inherited permissions of the parent temp directory.
	if err := restrictPathACL(dir, true); err != nil {
		cleanup()
		return nil, nil, errors.Wrap(err, "failed to restrict secret dir permissions")
	}

	fp := filepath.Join(dir, identity.NewID())
	// O_EXCL refuses to open an existing path, so a pre-planted file or symlink
	// at fp can never be followed or overwritten. On Windows the 0600 mode only
	// toggles the read-only attribute; access is governed by the inherited ACL.
	f, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	if _, err := f.Write(sm.sm.data); err != nil {
		f.Close()
		cleanup()
		return nil, nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return nil, nil, err
	}

	// Belt-and-suspenders: set an explicit protected DACL on the file too, in
	// case directory ACL inheritance is disabled on the host.
	if err := restrictPathACL(fp, false); err != nil {
		cleanup()
		return nil, nil, errors.Wrap(err, "failed to restrict secret file permissions")
	}

	return []mount.Mount{{
		Type:    "bind",
		Source:  fp,
		Options: []string{"ro"},
	}}, cleanup, nil
}

// restrictPathACL sets a protected DACL granting full control only to SYSTEM,
// Administrators and the daemon user. When inheritable, the ACEs also propagate
// to entries created inside a directory.
func restrictPathACL(path string, inheritable bool) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	flags := ""
	if inheritable {
		flags = "OICI"
	}
	// Build an SDDL DACL string granting full control to SYSTEM, the
	// Administrators group and the daemon user, and nobody else:
	//   D:      - this is a DACL
	//   P       - protected: do not inherit ACEs from the parent object
	//   (A;flags;FA;;;trustee) - an Access-Allowed ACE, where:
	//       flags   - "OICI" (object+container inherit) for dirs so children
	//                 inherit the ACL, empty for the leaf secret file
	//       FA      - File All access (full control)
	//       trustee - SY = Local System, BA = Builtin Administrators,
	//                 sid = the current (daemon) user's SID
	sddl := fmt.Sprintf("D:P(A;%[1]s;FA;;;SY)(A;%[1]s;FA;;;BA)(A;%[1]s;FA;;;%[2]s)", flags, sid)
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil,
	)
}

func currentUserSID() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}
