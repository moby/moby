// +build !386

package archutil

func i386Supported() error {
	return check(Binary386)
}
