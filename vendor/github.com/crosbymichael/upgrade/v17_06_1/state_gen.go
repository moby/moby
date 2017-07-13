// DO NOT EDIT
// This file has been auto-generated with go generate.

package v17_06_1

import (
	"time"

	"github.com/opencontainers/runc/libcontainer/configs" // 810190ceaa507aa2727d7ae6f4790c76ec150bd2 https://github.com/docker/runc
)

type State struct {
	ID                   string    `json:"id"`
	InitProcessPid       int       `json:"init_process_pid"`
	InitProcessStartTime string    `json:"init_process_start"`
	Created              time.Time `json:"created"`
	Config               struct {
		NoPivotRoot       bool               `json:"no_pivot_root"`
		ParentDeathSignal int                `json:"parent_death_signal"`
		Rootfs            string             `json:"rootfs"`
		Readonlyfs        bool               `json:"readonlyfs"`
		RootPropagation   int                `json:"rootPropagation"`
		Mounts            []*configs.Mount   `json:"mounts"`
		Devices           []*configs.Device  `json:"devices"`
		MountLabel        string             `json:"mount_label"`
		Hostname          string             `json:"hostname"`
		Namespaces        configs.Namespaces `json:"namespaces"`
		Capabilities      linuxCapabilities  `json:"capabilities"`
		Networks          []*configs.Network `json:"networks"`
		Routes            []*configs.Route   `json:"routes"`
		Cgroups           *struct {
			Name                         string `json:"name,omitempty"`
			Parent                       string `json:"parent,omitempty"`
			Path                         string `json:"path"`
			ScopePrefix                  string `json:"scope_prefix"`
			Paths                        map[string]string
			AllowAllDevices              *bool                     `json:"allow_all_devices,omitempty"`
			AllowedDevices               []*configs.Device         `json:"allowed_devices,omitempty"`
			DeniedDevices                []*configs.Device         `json:"denied_devices,omitempty"`
			Devices                      []*configs.Device         `json:"devices"`
			Memory                       int64                     `json:"memory"`
			MemoryReservation            int64                     `json:"memory_reservation"`
			MemorySwap                   int64                     `json:"memory_swap"`
			KernelMemory                 int64                     `json:"kernel_memory"`
			KernelMemoryTCP              int64                     `json:"kernel_memory_tcp"`
			CpuShares                    uint64                    `json:"cpu_shares"`
			CpuQuota                     int64                     `json:"cpu_quota"`
			CpuPeriod                    uint64                    `json:"cpu_period"`
			CpuRtRuntime                 int64                     `json:"cpu_rt_quota"`
			CpuRtPeriod                  uint64                    `json:"cpu_rt_period"`
			CpusetCpus                   string                    `json:"cpuset_cpus"`
			CpusetMems                   string                    `json:"cpuset_mems"`
			PidsLimit                    int64                     `json:"pids_limit"`
			BlkioWeight                  uint16                    `json:"blkio_weight"`
			BlkioLeafWeight              uint16                    `json:"blkio_leaf_weight"`
			BlkioWeightDevice            []*configs.WeightDevice   `json:"blkio_weight_device"`
			BlkioThrottleReadBpsDevice   []*configs.ThrottleDevice `json:"blkio_throttle_read_bps_device"`
			BlkioThrottleWriteBpsDevice  []*configs.ThrottleDevice `json:"blkio_throttle_write_bps_device"`
			BlkioThrottleReadIOPSDevice  []*configs.ThrottleDevice `json:"blkio_throttle_read_iops_device"`
			BlkioThrottleWriteIOPSDevice []*configs.ThrottleDevice `json:"blkio_throttle_write_iops_device"`
			Freezer                      configs.FreezerState      `json:"freezer"`
			HugetlbLimit                 []*configs.HugepageLimit  `json:"hugetlb_limit"`
			OomKillDisable               bool                      `json:"oom_kill_disable"`
			MemorySwappiness             memorySwappiness          `json:"memory_swappiness"`
			NetPrioIfpriomap             []*configs.IfPrioMap      `json:"net_prio_ifpriomap"`
			NetClsClassid                uint32                    `json:"net_cls_classid_u"`
		} `json:"cgroups"`
		AppArmorProfile string            `json:"apparmor_profile,omitempty"`
		ProcessLabel    string            `json:"process_label,omitempty"`
		Rlimits         []configs.Rlimit  `json:"rlimits,omitempty"`
		OomScoreAdj     int               `json:"oom_score_adj"`
		UidMappings     []configs.IDMap   `json:"uid_mappings"`
		GidMappings     []configs.IDMap   `json:"gid_mappings"`
		MaskPaths       []string          `json:"mask_paths"`
		ReadonlyPaths   []string          `json:"readonly_paths"`
		Sysctl          map[string]string `json:"sysctl"`
		Seccomp         *configs.Seccomp  `json:"seccomp"`
		NoNewPrivileges bool              `json:"no_new_privileges,omitempty"`
		Hooks           *configs.Hooks
		Version         string   `json:"version"`
		Labels          []string `json:"labels"`
		NoNewKeyring    bool     `json:"no_new_keyring"`
		Rootless        bool     `json:"rootless"`
	} `json:"config"`
	Rootless            bool                             `json:"rootless"`
	CgroupPaths         map[string]string                `json:"cgroup_paths"`
	NamespacePaths      map[configs.NamespaceType]string `json:"namespace_paths"`
	ExternalDescriptors []string                         `json:"external_descriptors,omitempty"`
}
