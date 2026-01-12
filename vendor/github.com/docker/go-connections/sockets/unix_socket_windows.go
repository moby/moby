package sockets

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// BasePermissions defines the default DACL, which allows Administrators
// and LocalSystem full access (similar to defaults used in [moby]);
//
// - D:P: DACL without inheritance (protected, (P)).
// - (A;;GA;;;BA): Allow full access (GA) for built-in Administrators (BA).
// - (A;;GA;;;SY); Allow full access (GA) for LocalSystem (SY).
// - Any other user is denied access.
//
// [moby]: https://github.com/moby/moby/blob/6b45c76a233b1b8b56465f76c21c09fd7920e82d/daemon/listeners/listeners_windows.go#L53-L59
const BasePermissions = "D:P(A;;GA;;;BA)(A;;GA;;;SY)"

// WithBasePermissions sets a default DACL, which allows Administrators
// and LocalSystem full access (similar to defaults used in [moby]);
//
// - D:P: DACL without inheritance (protected, (P)).
// - (A;;GA;;;BA): Allow full access (GA) for built-in Administrators (BA).
// - (A;;GA;;;SY); Allow full access (GA) for LocalSystem (SY).
// - Any other user is denied access.
//
// [moby]: https://github.com/moby/moby/blob/6b45c76a233b1b8b56465f76c21c09fd7920e82d/daemon/listeners/listeners_windows.go#L53-L59
func WithBasePermissions() SockOption {
	return withSDDL(BasePermissions)
}

// WithAdditionalUsersAndGroups modifies the socket file's DACL to grant
// access to additional users and groups.
//
// It sets [BasePermissions] on the socket path and grants the given additional
// users and groups to generic read (GR) and write (GW) access. It returns
// an error if no groups were given, when failing to resolve any of the
// additional users and groups, or when failing to apply the ACL.
func WithAdditionalUsersAndGroups(additionalUsersAndGroups []string) SockOption {
	return func(path string) error {
		if len(additionalUsersAndGroups) == 0 {
			return errors.New("no additional users specified")
		}
		sd, err := getSecurityDescriptor(additionalUsersAndGroups...)
		if err != nil {
			return fmt.Errorf("looking up SID: %w", err)
		}
		return withSDDL(sd)(path)
	}
}

// withSDDL applies the given SDDL to the socket. It returns an error
// when failing parse the SDDL, or if the DACL was defaulted.
//
// TODO(thaJeztah); this is not exported yet, as some of the checks may need review if they're not too opinionated.
func withSDDL(sddl string) SockOption {
	return func(path string) error {
		sd, err := windows.SecurityDescriptorFromString(sddl)
		if err != nil {
			return fmt.Errorf("parsing SDDL: %w", err)
		}
		dacl, defaulted, err := sd.DACL()
		if err != nil {
			return fmt.Errorf("extracting DACL: %w", err)
		}
		if dacl == nil || defaulted {
			// should never be hit with our [DefaultPermissions],
			// as it contains "D:" and "P" (protected, don't inherit).
			return errors.New("no DACL found in security descriptor or defaulted")
		}
		return windows.SetNamedSecurityInfo(
			path,
			windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
			nil, // do not change the owner
			nil, // do not change the owner
			dacl,
			nil,
		)
	}
}

// NewUnixSocket creates a new unix socket.
//
// It sets [BasePermissions] on the socket path and grants the given additional
// users and groups to generic read (GR) and write (GW) access. It returns
// an error when failing to resolve any of the additional users and groups,
// or when failing to apply the ACL.
func NewUnixSocket(path string, additionalUsersAndGroups []string) (net.Listener, error) {
	var opts []SockOption
	if len(additionalUsersAndGroups) > 0 {
		opts = append(opts, WithAdditionalUsersAndGroups(additionalUsersAndGroups))
	} else {
		opts = append(opts, WithBasePermissions())
	}
	return NewUnixSocketWithOpts(path, opts...)
}

// getSecurityDescriptor returns the DACL for the Unix socket.
//
// By default, it grants [BasePermissions], but allows for additional
// users and groups to get generic read (GR) and write (GW) access. It
// returns an error when failing to resolve any of the additional users
// and groups.
func getSecurityDescriptor(additionalUsersAndGroups ...string) (string, error) {
	sddl := BasePermissions

	// Grant generic read (GR) and write (GW) access to whatever
	// additional users or groups were specified.
	//
	// TODO(thaJeztah): should we fail on, or remove duplicates?
	for _, g := range additionalUsersAndGroups {
		sid, err := winio.LookupSidByName(strings.TrimSpace(g))
		if err != nil {
			return "", fmt.Errorf("looking up SID: %w", err)
		}
		sddl += fmt.Sprintf("(A;;GRGW;;;%s)", sid)
	}
	return sddl, nil
}

func listenUnix(path string) (net.Listener, error) {
	return net.Listen("unix", path)
}
