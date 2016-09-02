// +build !arm,!ppc64,!ppc64le,!s390x

package bridge

func ifrDataByte(b byte) int8 {
	return int8(b)
}
