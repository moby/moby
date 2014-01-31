// +build !linux !amd64

package lxc

func setHostname(hostname string) error {
	panic("Not supported on darwin")
}
