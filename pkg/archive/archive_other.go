//go:build !linux
// +build !linux

package archive // import "github.com/moby/moby/pkg/archive"

func getWhiteoutConverter(format WhiteoutFormat, inUserNS bool) (tarWhiteoutConverter, error) {
	return nil, nil
}
