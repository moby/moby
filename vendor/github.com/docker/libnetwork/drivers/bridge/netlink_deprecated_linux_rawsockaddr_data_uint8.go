//go:build arm || ppc64 || ppc64le || riscv64
// +build arm ppc64 ppc64le riscv64

package bridge

func ifrDataByte(b byte) uint8 {
	return uint8(b)
}
