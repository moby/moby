// +build linux,!seccomp

package seccomp // import "github.com/docker/docker/profiles/seccomp"

// DefaultProfile returns a nil pointer on unsupported systems.
func DefaultProfile() *Seccomp {
	return nil
}

// DefaultProfileWithoutUserNamespaces returns a nil pointer on unsupported systems.
func DefaultProfileWithoutUserNamespaces() *Seccomp {
	return nil
}
