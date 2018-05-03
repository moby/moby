// +build !windows

package distribution

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// GOARM should specify the variant of the CPU, in accordance with OCI standard.
// Valid values are v5, v6, v7. v8, etc.
var GOARM string
var vOrder32 = []string{"v7", "v6", "v5"}
var vOrder64 = []string{"v8"}

func init() {
	if runtime.GOARCH == "arm" || runtime.GOARCH == "arm64" {
		GOARM = getCPUVariant()
	} else {
		GOARM = ""
	}
}

// For Linux, the kernel has already detected the ABI, ISA and Features.
// So we don't need to access the ARM registers to detect platform information
// by ourselves. We can just parse these information from /proc/cpuinfo
func getCPUInfo(pattern string) (info string, err error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("getCPUInfo for OS %s not implemented", runtime.GOOS)
	}

	cpuinfo, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", err
	}
	defer cpuinfo.Close()

	// Start to Parse the Cpuinfo line by line. For SMP SoC, we parse
	// the first core is enough.
	scanner := bufio.NewScanner(cpuinfo)
	for scanner.Scan() {
		newline := scanner.Text()
		list := strings.Split(newline, ":")

		if len(list) > 1 && strings.EqualFold(strings.TrimSpace(list[0]), pattern) {
			return strings.TrimSpace(list[1]), nil
		}
	}

	// Check whether the scanner encountered errors
	err = scanner.Err()
	if err != nil {
		return "", err
	}

	return "", fmt.Errorf("getCPUInfo for pattern: %s not found", pattern)
}

func getCPUVariant() string {
	variant, err := getCPUInfo("Cpu architecture")
	if err != nil {
		logrus.Error("failure getting variant")
		return ""
	}

	switch variant {
	case "8":
		variant = "v8"
	case "7", "7M", "?(12)", "?(13)", "?(14)", "?(15)", "?(16)", "?(17)":
		variant = "v7"
	case "6", "6TEJ":
		variant = "v6"
	case "5", "5T", "5TE", "5TEJ":
		variant = "v5"
	case "4", "4T":
		variant = "v4"
	case "3":
		variant = "v3"
	default:
		variant = "unknown"
	}

	return variant
}

func getOrderOfCompatibility(ctx context.Context, arch string, variant string, order chan<- string) {
	var v string
	var i int
	if arch == "arm" { //arm32
		length := len(vOrder32)
		for i, v = range vOrder32 {
			if v == variant && i+1 < length {
				for k := i + 1; k < length; k++ {
					select {
					case <-ctx.Done():
						break
					case order <- vOrder32[k]:
					}
				}
				close(order)
				break
			}
		}
		if i == length-1 { //no match
			close(order)
		}
	} else { //arm64
		length := len(vOrder64)
		for i, v = range vOrder64 {
			if v == variant && i+1 < length {
				for k := i + 1; k < length; k++ {
					select {
					case <-ctx.Done():
						break
					case order <- vOrder64[k]:
					}
				}
				close(order)
				break
			}
		}
		if i == length-1 { //no match
			close(order)
		}

	}
}
