package binfmt_misc

import (
	"strings"
	"sync"

	"github.com/containerd/containerd/platforms"
)

var once sync.Once
var arr []string

func SupportedPlatforms() []string {
	once.Do(func() {
		def := platforms.DefaultString()
		arr = append(arr, def)

		if p := "linux/amd64"; def != p && amd64Supported() {
			arr = append(arr, p)
		}
		if p := "linux/arm64"; def != p && arm64Supported() {
			arr = append(arr, p)
		}
		if !strings.HasPrefix(def, "linux/arm/") && armSupported() {
			arr = append(arr, "linux/arm/v7", "linux/arm/v6")
		} else if def == "linux/arm/v7" {
			arr = append(arr, "linux/arm/v6")
		}
	})

	return arr
}
