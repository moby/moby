//go:build !linux && !windows && !freebsd

package osl

type Namespace struct{}

func (n *Namespace) Destroy() error { return nil }

// GetSandboxForExternalKey returns sandbox object for the supplied path
func GetSandboxForExternalKey(path string, key string) (*Namespace, error) {
	return nil, nil
}
