package sysinfo

import (
	"path"
	"testing"
)

type ExistsTests struct {
	name   string
	atPath string
}

type SwapLimitTests struct {
	name  string
	resrc ResourceSupport
	want  bool
}

var shouldExist = []ExistsTests{
	/* Presumes swap-related files exist on kernels compiled w/CONFIG_SWAP=y */
	{
		name:   "Should detect that swappiness fs link does exist",
		atPath: "/proc/sys/vm/swappiness",
	},
	{
		name:   "Should detect that swapon program does exist",
		atPath: "/sbin/swapon",
	},
	{
		name:   "Should detect that swaps fs link does exist",
		atPath: "/proc/swaps",
	},
}

var shouldNotExist = []ExistsTests{
	/* Presumes swap-related files DON'T exist on kernels compiled w/CONFIG_SWAP=0 */
	{
		name:   "Should detect that swappiness fs link does NOT exist",
		atPath: "!/proc/sys/vm/swappiness",
	},
	{
		name:   "Should detect that swapon program does NOT exist",
		atPath: "!/sbin/swapon",
	},
	{
		name:   "Should detect that swaps fs link does NOT exist",
		atPath: "!/proc/swaps",
	},
}

var shouldDetectSwap = []SwapLimitTests{
	/* Root cgroup (i.e., no memory.swap.max) + case; '0::/' detected in /proc/self/cgroup*/
	{
		name: "Should detect files created when the kernel's compiled w/CONFIG_SWAP=y",
		resrc: &SwapSupport{
			procSelfCg: "testdata/proc/self/mock_root_cgroup",
			swapon:     "testdata/sbin/swapon",
			swaps:      "testdata/proc/swaps",
			swappiness: "testdata/proc/sys/vm/swappiness",
		},
		want: true,
	},
	{ /* Subcgroup + case; '0::/desystemded' detected in /proc/self/cgroup*/
		name: "Should detect memory.swap.max v2 interface file for non-root cgroups",
		resrc: &SwapSupport{
			procSelfCg: "testdata/proc/self/mock_sub_cgroup",
			mntPoint:   "testdata/sys/fs/cgroup",
		},
		want: true,
	},
}

func TestExists(t *testing.T) {
	for _, tt := range shouldExist {
		t.Run(tt.name, func(t *testing.T) {
			if !exists(path.Join("testdata", tt.atPath)) {
				t.Fatal("Failed to detect swap-related files that are expected to exist")
			}
		})
	}
}

func TestExistsNot(t *testing.T) {
	for _, tt := range shouldNotExist {
		t.Run(tt.name, func(t *testing.T) {
			if exists(path.Join("testdata", tt.atPath)) {
				t.Fatal("Failed to detect the absense of swap-related files")
			}
		})
	}
}

func TestGetSwapLimitV2(t *testing.T) {
	for _, tt := range shouldDetectSwap {
		t.Run(tt.name, func(t *testing.T) {
			if got := getSwapLimitV2(tt.resrc); got != tt.want {
				t.Errorf("getSwapLimitV2() = %v, want %v", got, tt.want)
			}
		})
	}
}
