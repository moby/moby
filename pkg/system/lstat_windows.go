package system // import "github.com/docker/docker/pkg/system"

import "os"

// Lstat calls os.Lstat to get a fileinfo interface back.
// This is then copied into our own locally defined structure.
//
// Deprecated: this function is only used internally, and will be removed in the next release.
func Lstat(path string) (*StatT, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	return fromStatT(&fi)
}
