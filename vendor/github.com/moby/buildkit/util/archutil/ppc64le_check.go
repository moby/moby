//go:build !ppc64le
// +build !ppc64le

package archutil

func ppc64leSupported() error {
	return check(Binaryppc64le)
}
