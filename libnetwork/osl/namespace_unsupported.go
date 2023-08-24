//go:build !linux && !windows && !freebsd

package osl

type Namespace struct{}

func (n *Namespace) Destroy() error { return nil }

// GC triggers garbage collection of namespace path right away
// and waits for it.
func GC() {
}

// GetSandboxForExternalKey returns sandbox object for the supplied path
func GetSandboxForExternalKey(path string, key string) (*Namespace, error) {
	return nil, nil
}
