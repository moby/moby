package sysinfo

import (
	"io/ioutil"
	"os"
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libcontainer/cgroups"
)

type SysInfo struct {
	MemoryLimit            bool
	SwapLimit              bool
	IPv4ForwardingDisabled bool
	AppArmor               bool
}

func New(quiet bool) *SysInfo {
	sysInfo := &SysInfo{}
	if cgroupMemoryMountpoint, err := cgroups.FindCgroupMountpoint("memory"); err != nil {
		if !quiet {
			log.Printf("WARNING: %s\n", err)
		}
	} else {
		_, err1 := ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.limit_in_bytes"))
		_, err2 := ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.soft_limit_in_bytes"))
		sysInfo.MemoryLimit = err1 == nil && err2 == nil
		if !sysInfo.MemoryLimit && !quiet {
			log.Printf("WARNING: Your kernel does not support cgroup memory limit.")
		}

		_, err = ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.memsw.limit_in_bytes"))
		sysInfo.SwapLimit = err == nil
		if !sysInfo.SwapLimit && !quiet {
			log.Printf("WARNING: Your kernel does not support cgroup swap limit.")
		}
	}

	// Check if AppArmor seems to be enabled on this system.
	if _, err := os.Stat("/sys/kernel/security/apparmor"); os.IsNotExist(err) {
		sysInfo.AppArmor = false
	} else {
		sysInfo.AppArmor = true
	}
	return sysInfo
}
