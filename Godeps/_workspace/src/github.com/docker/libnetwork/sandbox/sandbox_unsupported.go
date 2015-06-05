// +build !linux

package sandbox

import "errors"

var (
	ErrNotImplemented = errors.New("not implemented")
)

// NewSandbox provides a new sandbox instance created in an os specific way
// provided a key which uniquely identifies the sandbox
func NewSandbox(key string) (Sandbox, error) {
	return nil, ErrNotImplemented
}
