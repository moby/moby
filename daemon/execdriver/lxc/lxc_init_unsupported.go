// +build !linux !amd64 !arm

package lxc

func setHostname(hostname string) error {
	panic("Not supported on darwin")
}
