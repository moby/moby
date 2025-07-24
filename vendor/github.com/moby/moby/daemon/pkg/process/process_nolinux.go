//go:build !linux

package process

func zombie(pid int) (bool, error) {
	return false, nil
}
