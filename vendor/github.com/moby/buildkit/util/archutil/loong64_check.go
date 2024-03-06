//go:build !loong64
// +build !loong64

package archutil

func loong64Supported() (string, error) {
	return check("loong64", Binaryloong64)
}
