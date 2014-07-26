// +build linux

package utils

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/parsers/kernel"
)

var (
	procMounts = "/proc/mounts"

	possibleKernelConfigs = func() []string {
		configs := make([]string, 0, 5)
		config := os.Getenv("CONFIG")
		if len(config) > 0 {
			configs = append(configs, config)
		}

		configs = append(configs,
			"/proc/config.gz",
			"/usr/src/linux/.config",
		)

		if kv, err := kernel.GetKernelVersion(); err == nil {
			configs = append(configs,
				fmt.Sprintf("/boot/config-%s", kv),
				fmt.Sprintf("/usr/src/linux-%s/.config", kv))
		}
		return configs
	}
)

type checkConfigError []string

func (err checkConfigError) Error() string {
	return fmt.Sprintf("Missing kernel configs: %s", strings.Join([]string(err), " "))
}

func checkCGroupConfig() error {
	cgroups := []string{"cpu", "cpuacct", "cpuset", "devices", "freezer", "memory"}
	cgroupRegex := regexp.MustCompile(fmt.Sprintf(`^\S+\s+(\S+)\s+cgroup\s+.*[, ](?:%s)[, ]`, strings.Join(cgroups, "|")))
	b, err := ioutil.ReadFile(procMounts)
	if err != nil {
		return fmt.Errorf("Cannot check kernel config: %v", err)
	}

	var (
		cgroupDir          string
		cgroupSubsystemDir string
	)
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if sm := cgroupRegex.FindSubmatch(line); len(sm) >= 2 {
			cgroupSubsystemDir = string(sm[1])
			cgroupDir = filepath.Dir(cgroupSubsystemDir)
			break
		}
	}

	if len(cgroupDir) == 0 {
		return fmt.Errorf("Cannot detect cgroup dir")
	}

	properlyMounted := false
	for _, subdir := range cgroups {
		st, err := os.Stat(filepath.Join(cgroupDir, subdir))
		if !os.IsNotExist(err) && st != nil && st.IsDir() {
			properlyMounted = true
			break
		}
	}
	if !properlyMounted {
		var s string
		if len(cgroupSubsystemDir) > 0 {
			s = "Single cgroup hierarchy mountpoint"
		} else {
			s = "Non-existent cgroup hierarchy mountpoint"
		}
		return fmt.Errorf("%s: (see https://github.com/tianon/cgroupfs-mount)", s)
	}

	return nil
}

func checkKernelConfig() error {

	possibleConfigs := possibleKernelConfigs()

	r := bufio.NewReader(nil)
	for _, config := range possibleConfigs {
		var (
			f   *os.File
			gzr *gzip.Reader
			err error
		)

		f, err = os.Open(config)
		if err != nil {
			continue
		}

		gz := strings.HasSuffix(config, ".gz")
		if gz {
			gzr, err = gzip.NewReader(f)
			if err != nil {
				log.Debugf("Cannot open kernel config %s: %v", config, err)
				f.Close()
				continue
			}
		}

		// From here on, we assume we found THE kernel config.
		// So we'll return errors and not move on to the next config path.
		defer func() {
			if gz {
				gzr.Close()
			}
			f.Close()
		}()

		var checkConfigErr checkConfigError

		for _, fl := range []string{
			"NAMESPACES",
			"NET_NS",
			"PID_NS",
			"IPC_NS",
			"UTS_NS",
			"DEVPTS_MULTIPLE_INSTANCES",
			"CGROUPS",
			"CGROUP_CPUACCT",
			"CGROUP_DEVICE",
			"CGROUP_FREEZER",
			"CGROUP_SCHED",
			"MACVLAN",
			"VETH",
			"BRIDGE",
			"NF_NAT_IPV4",
			"IP_NF_TARGET_MASQUERADE",
			"NETFILTER_XT_MATCH_ADDRTYPE",
			"NETFILTER_XT_MATCH_CONNTRACK",
			"NF_NAT",
			"NF_NAT_NEEDED",
		} {
			f.Seek(0, 0)
			if gz {
				gzr.Close()
				gzr, err = gzip.NewReader(f)
				if err != nil {
					return fmt.Errorf("Cannot open kernel config %s: %v", config, err)
				}
				r.Reset(gzr)
			} else {
				r.Reset(f)
			}
			s := fmt.Sprintf(`CONFIG_%s=[ym]`, regexp.QuoteMeta(fl))
			if !regexp.MustCompile(s).MatchReader(r) {
				checkConfigErr = append(checkConfigErr, fl)
			}
		}

		if len(checkConfigErr) > 0 {
			return checkConfigErr
		}

		return nil
	}

	return fmt.Errorf("Cannot find kernel config. Looked at %s. Try specyfing CONFIG=/path/to/kernel/config", strings.Join(possibleConfigs, ", "))
}

func checkApparmor() error {
	if b, err := ioutil.ReadFile("/sys/module/apparmor/parameters/enabled"); err == nil && len(b) == 1 && b[0] == 'Y' {
		if _, err := exec.LookPath("apparmor_parser"); err != nil {
			s := "enabled but apparmor_parser missing"
			if _, err := exec.LookPath("apt-get"); err == nil {
				return fmt.Errorf("%s: (use 'apt-get install apparmor' to fix this)", s)
			}
			if _, err := exec.LookPath("yum"); err == nil {
				return fmt.Errorf("%s: (your best bet is 'yum install apparmor-parser' to fix this)", s)
			}
			return fmt.Errorf("%s: (look for an 'apparmor' package for your distribution)", s)
		}
	}
	return nil
}

func CheckConfig() error {
	if err := checkKernelConfig(); err != nil {
		return fmt.Errorf("kernel config: %v", err)
	}
	if err := checkCGroupConfig(); err != nil {
		return fmt.Errorf("cgroup config: %v", err)
	}
	if err := checkApparmor(); err != nil {
		return fmt.Errorf("apparmor: %v", err)
	}

	return nil
}
