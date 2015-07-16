// +build !arm,!ppc64

package netlink

func ifrDataByte(b byte) int8 {
	return int8(b)
}
