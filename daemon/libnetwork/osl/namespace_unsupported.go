//go:build !linux && !windows && !freebsd

package osl

import "errors"

type Namespace struct{}

func (n *Namespace) Destroy() error { return nil }

func (n *Namespace) InvokeFunc(f func()) error {
	if f != nil {
		f()
	}
	return nil
}

// ErrNotImplemented is for platforms which don't implement sandbox.
var ErrNotImplemented = errors.New("not implemented")

// NewSandbox provides a new sandbox instance created in an os specific way.
func NewSandbox(key string, osCreate, isRestore bool) (*Namespace, error) {
	return nil, ErrNotImplemented
}

// GenerateKey generates a sandbox key based on the passed container id.
func GenerateKey(containerID string) string {
	return ""
}

// GetSandboxForExternalKey returns sandbox object for the supplied path
func GetSandboxForExternalKey(path string, key string) (*Namespace, error) {
	return nil, nil
}
