package kernel // import "github.com/docker/docker/pkg/parsers/kernel"

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// VersionInfo holds information about the kernel.
type VersionInfo struct {
	kvi   string // Version of the kernel (e.g. 6.1.7601.17592 -> 6)
	major int    // Major part of the kernel version (e.g. 6.1.7601.17592 -> 1)
	minor int    // Minor part of the kernel version (e.g. 6.1.7601.17592 -> 7601)
	build int    // Build number of the kernel version (e.g. 6.1.7601.17592 -> 17592)
}

func (k *VersionInfo) String() string {
	return fmt.Sprintf("%d.%d %d (%s)", k.major, k.minor, k.build, k.kvi)
}

// GetKernelVersion gets the current kernel version.
func GetKernelVersion() (*VersionInfo, error) {

	KVI := &VersionInfo{"Unknown", 0, 0, 0}

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return KVI, err
	}
	defer k.Close()

	blex, _, err := k.GetStringValue("BuildLabEx")
	if err != nil {
		return KVI, err
	}
	KVI.kvi = blex

	// Important - docker.exe MUST be manifested for this API to return
	// the correct information.
	dwVersion, err := windows.GetVersion()
	if err != nil {
		return KVI, err
	}

	KVI.major = int(dwVersion & 0xFF)
	KVI.minor = int((dwVersion & 0XFF00) >> 8)
	KVI.build = int((dwVersion & 0xFFFF0000) >> 16)

	return KVI, nil
}
