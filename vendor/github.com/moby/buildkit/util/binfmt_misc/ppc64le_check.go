// +build !ppc64le

package binfmt_misc

func ppc64leSupported() error {
	return check(Binaryppc64le)
}
