//go:build !riscv64
// +build !riscv64

package archutil

func riscv64Supported() error {
	return check(Binaryriscv64)
}
