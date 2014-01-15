package sysinfo

import (
	"github.com/dotcloud/docker/cgroups"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"log"
	"os"
	"path"
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

	content, err3 := ioutil.ReadFile("/proc/sys/net/ipv4/ip_forward")
	sysInfo.IPv4ForwardingDisabled = err3 != nil || len(content) == 0 || content[0] != '1'
	if sysInfo.IPv4ForwardingDisabled && !quiet {
		log.Printf("WARNING: IPv4 forwarding is disabled.")
	}

	// Check if AppArmor seems to be enabled on this system.
	if _, err := os.Stat("/sys/kernel/security/apparmor"); os.IsNotExist(err) {
		utils.Debugf("/sys/kernel/security/apparmor not found; assuming AppArmor is not enabled.")
		sysInfo.AppArmor = false
	} else {
		utils.Debugf("/sys/kernel/security/apparmor found; assuming AppArmor is enabled.")
		sysInfo.AppArmor = true
	}
	return sysInfo
}
