//go:build amd64
// +build amd64

package archutil

import (
	archvariant "github.com/tonistiigi/go-archvariant"
)

func amd64Supported() (string, error) {
	return archvariant.AMD64Variant(), nil
}
