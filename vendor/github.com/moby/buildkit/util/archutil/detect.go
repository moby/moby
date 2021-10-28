package archutil

import (
	"strings"
	"sync"

	"github.com/containerd/containerd/platforms"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

var mu sync.Mutex
var arr []string

func SupportedPlatforms(noCache bool) []string {
	mu.Lock()
	defer mu.Unlock()
	if !noCache && arr != nil {
		return arr
	}
	def := defaultPlatform()
	arr = append([]string{}, def)
	if p := "linux/amd64"; def != p && amd64Supported() == nil {
		arr = append(arr, p)
	}
	if p := "linux/arm64"; def != p && arm64Supported() == nil {
		arr = append(arr, p)
	}
	if p := "linux/riscv64"; def != p && riscv64Supported() == nil {
		arr = append(arr, p)
	}
	if p := "linux/ppc64le"; def != p && ppc64leSupported() == nil {
		arr = append(arr, p)
	}
	if p := "linux/s390x"; def != p && s390xSupported() == nil {
		arr = append(arr, p)
	}
	if p := "linux/386"; def != p && i386Supported() == nil {
		arr = append(arr, p)
	}
	if p := "linux/mips64le"; def != p && mips64leSupported() == nil {
		arr = append(arr, p)
	}
	if p := "linux/mips64"; def != p && mips64Supported() == nil {
		arr = append(arr, p)
	}
	if !strings.HasPrefix(def, "linux/arm/") && armSupported() == nil {
		arr = append(arr, "linux/arm/v7", "linux/arm/v6")
	} else if def == "linux/arm/v7" {
		arr = append(arr, "linux/arm/v6")
	}
	return arr
}

func Check(pp ocispecs.Platform) bool {
	p := platforms.Format(pp)
	if p == "linux/amd64" && amd64Supported() == nil {
		return true
	}
	if p == "linux/arm64" && arm64Supported() == nil {
		return true
	}
	if p == "linux/riscv64" && riscv64Supported() == nil {
		return true
	}
	if p == "linux/ppc64le" && ppc64leSupported() == nil {
		return true
	}
	if p == "linux/s390x" && s390xSupported() == nil {
		return true
	}
	if p == "linux/386" && i386Supported() == nil {
		return true
	}
	if p == "linux/mips64le" && mips64leSupported() == nil {
		return true
	}
	if p == "linux/mips64" && mips64Supported() == nil {
		return true
	}
	if !strings.HasPrefix(p, "linux/arm/") && armSupported() == nil {
		return true
	}

	return false
}

//WarnIfUnsupported validates the platforms and show warning message if there is,
//the end user could fix the issue based on those warning, and thus no need to drop
//the platform from the candidates.
func WarnIfUnsupported(pfs []string) {
	def := defaultPlatform()
	for _, p := range pfs {
		if p != def {
			if p == "linux/amd64" {
				if err := amd64Supported(); err != nil {
					printPlatformWarning(p, err)
				}
			}
			if p == "linux/arm64" {
				if err := arm64Supported(); err != nil {
					printPlatformWarning(p, err)
				}
			}
			if p == "linux/riscv64" {
				if err := riscv64Supported(); err != nil {
					printPlatformWarning(p, err)
				}
			}
			if p == "linux/ppc64le" {
				if err := ppc64leSupported(); err != nil {
					printPlatformWarning(p, err)
				}
			}
			if p == "linux/s390x" {
				if err := s390xSupported(); err != nil {
					printPlatformWarning(p, err)
				}
			}
			if p == "linux/386" {
				if err := i386Supported(); err != nil {
					printPlatformWarning(p, err)
				}
			}
			if p == "linux/mips64le" {
				if err := mips64leSupported(); err != nil {
					printPlatformWarning(p, err)
				}
			}
			if p == "linux/mips64" {
				if err := mips64Supported(); err != nil {
					printPlatformWarning(p, err)
				}
			}
			if strings.HasPrefix(p, "linux/arm/v6") || strings.HasPrefix(p, "linux/arm/v7") {
				if err := armSupported(); err != nil {
					printPlatformWarning(p, err)
				}
			}
		}
	}
}

func defaultPlatform() string {
	return platforms.Format(platforms.Normalize(platforms.DefaultSpec()))
}

func printPlatformWarning(p string, err error) {
	if strings.Contains(err.Error(), "exec format error") {
		logrus.Warnf("platform %s cannot pass the validation, kernel support for miscellaneous binary may have not enabled.", p)
	} else if strings.Contains(err.Error(), "no such file or directory") {
		logrus.Warnf("platforms %s cannot pass the validation, '-F' flag might have not set for 'archutil'.", p)
	} else {
		logrus.Warnf("platforms %s cannot pass the validation: %s", p, err.Error())
	}
}
