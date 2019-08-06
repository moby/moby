// +build !386

package binfmt_misc

func i386Supported() error {
	return check(Binary386)
}
