//go:build !ppc64le
// +build !ppc64le

package archutil

func ppc64leSupported() (string, error) {
	return check("ppc64le", Binaryppc64le)
}
