// +build !linux !cgo static_build !journald linux,arm

package journald

func (s *journald) Close() error {
	return nil
}
