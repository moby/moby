// +build !linux !cgo static_build !journald

package journald // import "github.com/moby/moby/daemon/logger/journald"

func (s *journald) Close() error {
	return nil
}
