// +build !arm,!ppc64

package bridge

func ifrDataByte(b byte) int8 {
	return int8(b)
}
