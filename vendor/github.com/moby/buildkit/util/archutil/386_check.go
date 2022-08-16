//go:build !386
// +build !386

package archutil

func i386Supported() (string, error) {
	return check("386", Binary386)
}
