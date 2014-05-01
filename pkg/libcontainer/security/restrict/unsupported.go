// +build !linux

package restrict

import "fmt"

func Restrict(rootfs, empty string) error {
	return fmt.Errorf("not supported")
}
