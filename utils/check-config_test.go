// +build linux

package utils

import (
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckKernelConfig(t *testing.T) {
	var (
		backup  = possibleKernelConfigs
		configs = map[string][]byte{
			"noMatch": []byte(`CONFIG_DQL=y
CONFIG_NLATTR=y
CONFIG_ARCH_HAS_ATOMIC64_DEC_IF_POSITIVE=y
CONFIG_AVERAGE=y
CONFIG_CORDIC=m
# CONFIG_DDR is not set
CONFIG_OID_REGISTRY=y
CONFIG_UCS2_STRING=y
CONFIG_FONT_SUPPORT=y
# CONFIG_FONTS is not set
CONFIG_FONT_8x8=y
CONFIG_FONT_8x16=y`),
			"fewMatches": []byte(`CONFIG_NLATTR=y
CONFIG_NAMESPACES=y
CONFIG_AVERAGE=y
CONFIG_NET_NS=m
# CONFIG_FONTS is not set
`),
			"allMatches": []byte(`CONFIG_FONT_SUPPORT=y
CONFIG_NAMESPACES=y
CONFIG_OID_REGISTRY=y
CONFIG_NET_NS=y
CONFIG_PID_NS=y
CONFIG_IPC_NS=y
CONFIG_DQL=y
CONFIG_UTS_NS=y
CONFIG_DEVPTS_MULTIPLE_INSTANCES=m
CONFIG_CGROUPS=y
CONFIG_CGROUP_CPUACCT=y
CONFIG_CGROUP_DEVICE=y
CONFIG_CGROUP_FREEZER=y
CONFIG_CGROUP_SCHED=y
CONFIG_MACVLAN=y
CONFIG_VETH=y
CONFIG_BRIDGE=y
CONFIG_NF_NAT_IPV4=y
CONFIG_FONT_8x8=y
CONFIG_IP_NF_TARGET_MASQUERADE=y
CONFIG_NETFILTER_XT_MATCH_ADDRTYPE=y
CONFIG_NETFILTER_XT_MATCH_CONNTRACK=y
CONFIG_NF_NAT=y
CONFIG_NF_NAT_NEEDED=y`),
		}
	)

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		possibleKernelConfigs = backup
		os.RemoveAll(dir)
	}()

	var config string

	testConfigs := func(compressed bool) error {
		for name, data := range configs {
			if !compressed {
				if err := ioutil.WriteFile(config, data, 0600); err != nil {
					return err
				}
			} else {
				f, err := os.Create(config)
				if err != nil {
					return err
				}
				defer f.Close()
				w := gzip.NewWriter(f)
				defer w.Close()
				if _, err := w.Write(data); err != nil {
					return err
				}
				w.Flush()
			}

			if err := checkKernelConfig(); (err == nil) != (name == "allMatches") {
				var expected string
				if err != nil {
					expected = "no"
				} else {
					expected = "an"
				}
				return fmt.Errorf("Expected %s error(s) for %s, but got: %#v", expected, name, err)
			}
		}

		return nil
	}

	// test text kernel configs
	config = filepath.Join(dir, "config")
	possibleKernelConfigs = func() []string {
		// test skipping nonexistent files
		return []string{filepath.Join(dir, "nonexistent"), config}
	}
	if err := testConfigs(false); err != nil {
		t.Errorf("text kernel config: %v", err)
	}

	// test gzipped kernel configs
	config = filepath.Join(dir, "config.gz")
	possibleKernelConfigs = func() []string {
		// test skipping corrupt gzip file
		corrupt := filepath.Join(dir, "corrupt.gz")
		f, _ := os.Create(corrupt)
		if f != nil {
			f.Close()
		}
		return []string{corrupt, config}
	}
	if err := testConfigs(true); err != nil {
		t.Errorf("gzipped kernel config: %v", err)
	}
}

func TestCGroupConfig(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	var (
		backup       = procMounts
		cGroupMounts = []byte(fmt.Sprintf(`rootfs / rootfs rw,size=2015968k,nr_inodes=503992 0 0
tmpfs / tmpfs rw,relatime,size=3648284k 0 0
cgroup /sys/fs/cgroup tmpfs rw,relatime,mode=755 0 0
cgroup %s/cpuset cgroup rw,relatime,cpuset 0 0
cgroup %s/cpu cgroup rw,relatime,cpu 0 0`, dir, dir))
		noCGroupMounts = []byte(`rootfs / rootfs rw,size=2015968k,nr_inodes=503992 0 0
tmpfs / tmpfs rw,relatime,size=3648284k 0 0`)
	)

	defer func() {
		procMounts = backup
		os.RemoveAll(dir)
	}()

	procMounts = filepath.Join(dir, "nonexistent")
	if err := checkCGroupConfig(); err == nil {
		t.Error("Expected an error reading nonexistent mounts file but got nil")
	}

	procMounts = filepath.Join(dir, "mounts")
	if err := ioutil.WriteFile(procMounts, noCGroupMounts, 0600); err != nil {
		t.Error("Could not write file %s: %v", procMounts, err)
	}
	if err := checkCGroupConfig(); err == nil {
		t.Error("Expected an error reading mounts without cgroups, but got nil")
	}

	if err := ioutil.WriteFile(procMounts, cGroupMounts, 0600); err != nil {
		t.Error("Could not write file %s: %v", procMounts, err)
	}
	f, _ := os.Create(filepath.Join(dir, "cpuset"))
	if f != nil {
		f.Close()
	}
	if err := checkCGroupConfig(); err == nil {
		t.Error("Expected an error reading mounts with invalid cgroup mountpoints, but got nil")
	}
	if err := os.MkdirAll(filepath.Join(dir, "cpu"), 0700); err != nil {
		t.Error(err)
	}
	if err := checkCGroupConfig(); err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
}
