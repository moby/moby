package docker

import (
	"bytes"
	"strconv"
	"strings"
	"syscall"
)

func getKernelVersion() (*KernelVersionInfo, error) {
	var (
		uts                  syscall.Utsname
		flavor               string
		kernel, major, minor int
		err                  error
	)

	if err := syscall.Uname(&uts); err != nil {
		return nil, err
	}

	release := make([]byte, len(uts.Release))

	i := 0
	for _, c := range uts.Release {
		release[i] = byte(c)
		i++
	}

	// Remove the \x00 from the release for Atoi to parse correctly
	release = release[:bytes.IndexByte(release, 0)]

	tmp := strings.SplitN(string(release), "-", 2)
	tmp2 := strings.SplitN(tmp[0], ".", 3)

	if len(tmp2) > 0 {
		kernel, err = strconv.Atoi(tmp2[0])
		if err != nil {
			return nil, err
		}
	}

	if len(tmp2) > 1 {
		major, err = strconv.Atoi(tmp2[1])
		if err != nil {
			return nil, err
		}
	}

	if len(tmp2) > 2 {
		minor, err = strconv.Atoi(tmp2[2])
		if err != nil {
			return nil, err
		}
	}

	if len(tmp) == 2 {
		flavor = tmp[1]
	} else {
		flavor = ""
	}

	return &KernelVersionInfo{
		Kernel: kernel,
		Major:  major,
		Minor:  minor,
		Flavor: flavor,
	}, nil
}
