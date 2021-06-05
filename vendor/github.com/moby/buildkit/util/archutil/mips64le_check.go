// +build !mips64le

package archutil

func mips64leSupported() error {
	return check(Binarymips64le)
}
