//go:build !ppc64
// +build !ppc64

package archutil

func ppc64Supported() (string, error) {
	return check("ppc64", Binaryppc64)
}
