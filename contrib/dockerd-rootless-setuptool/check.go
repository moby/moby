package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

func runCheckCmd(cmd *cobra.Command, args []string) {
	check()
	logrus.Info("System requirements are satisfied")
}

// check is called by several commands
func check() {
	// check user
	if os.Geteuid() == 0 {
		logrus.Fatal("Must be executed as a non-root user")
	}

	// check HOME
	home := os.Getenv("HOME")
	if home == "" {
		logrus.Fatalf("$HOME must be set")
	}
	if err := unix.Access(home, unix.W_OK); err != nil {
		logrus.WithError(err).Fatalf("$HOME (%s) must be writable", home)
	}
	if binDir, err := detectBinDir(); err != nil {
		logrus.WithError(err).Fatalf("Cannot detect the binary directory")
	} else {
		logrus.Debugf("Detected bin dir: %s", binDir)
	}

	// check rootful Docker
	if !ignoreRootful && unix.Access(rootfulDockerSock, unix.W_OK) == nil {
		logrus.Fatalf("Aborting because rootful Docker (%s) is running and accessible. Set --ignore-rootful to ignore.", rootfulDockerSock)
	}

	if userSystemdAvailable() {
		if err := unix.Access(os.Getenv("XDG_RUNTIME_DIR"), unix.W_OK); err != nil {
			logrus.WithError(err).Fatal("systemd was detected but $XDG_RUNTIME_DIR does not exist or is not writable\n" +
				"Hint: this could happen if you changed users with 'su' or 'sudo'. To work around this:\n" +
				"- try again by first running with root privileges 'loginctl enable-linger <user>' where <user> is the unprivileged user and export XDG_RUNTIME_DIR to the value of RuntimePath as shown by 'loginctl show-user <user>'\n" +
				"- or simply log back in as the desired unprivileged user (ssh works for remote machines, machinectl shell works for the local machine)\n")
		}
	}

	var insts []Instruction
	insts = append(insts, instUidmap()...)
	insts = append(insts, instIptables()...)
	insts = append(insts, instSysctl()...)
	insts = append(insts, instSubid()...)
	printInsts(insts)
}

func printInsts(insts []Instruction) {
	if len(insts) == 0 {
		return
	}
	hasEssential := false
	const (
		shellInstHeader = "# Missing system requirements. Run following commands to install the requirements.\ncat <<EOF | sudo sh -x\n"
		shellInstFooter = "EOF\n"
	)
	shellInst := shellInstHeader
	humanInst := ""
	for _, inst := range insts {
		if inst.Essential {
			hasEssential = true
		}
		if inst.Shell {
			shellInst += inst.Instruction + "\n"
		} else {
			humanInst += inst.Instruction + "\n"
		}
	}
	if shellInst == shellInstHeader {
		// no shell inst was appended
		shellInst = ""
	} else {
		shellInst += shellInstFooter
	}
	instText := humanInst + "\n" + shellInst
	if hasEssential {
		logrus.Fatal(instText)
	} else {
		logrus.Warn(instText)
	}
}

func detectBinDir() (string, error) {
	p, err := exec.LookPath(dockerdRootlessSh)
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

var (
	userSystemdAvailableOnce sync.Once
	userSystemdAvailableVal  bool
)

func userSystemdAvailable() bool {
	userSystemdAvailableOnce.Do(func() {
		err := exec.Command("systemctl", "--user", "daemon-reload").Run()
		userSystemdAvailableVal = err == nil
	})
	return userSystemdAvailableVal
}

// PackageManager is for package managers like apt-get and dnf
type PackageManager int

const (
	// PackageManagerUnknown stands for unknown package manager
	PackageManagerUnknown PackageManager = iota
	// PackageManagerAptGet is for apt-get
	PackageManagerAptGet
	// PackageManagerDnf is for dnf
	PackageManagerDnf
	// PackageManagerYum is for yum
	PackageManagerYum
)

var (
	getPackageManagerOnce sync.Once
	getPackageManagerVal  PackageManager = PackageManagerUnknown
)

func getPackageManager() PackageManager {
	getPackageManagerOnce.Do(func() {
		if _, err := exec.LookPath("apt-get"); err == nil {
			getPackageManagerVal = PackageManagerAptGet
		} else if _, err := exec.LookPath("dnf"); err == nil {
			getPackageManagerVal = PackageManagerDnf
		} else if _, err := exec.LookPath("yum"); err == nil {
			getPackageManagerVal = PackageManagerYum
		}
	})
	return getPackageManagerVal
}

// Instruction instructs system requirements
type Instruction struct {
	Essential bool
	Shell     bool
	// Instruction is a shell command if Shell is true.
	// Otherwise human-readable text.
	Instruction string
}

func instUidmap() []Instruction {
	if _, err := exec.LookPath("newuidmap"); err != nil {
		switch getPackageManager() {
		case PackageManagerAptGet:
			return []Instruction{
				{
					Essential:   true,
					Shell:       true,
					Instruction: "apt-get install -y uidmap",
				},
			}
		case PackageManagerDnf:
			return []Instruction{
				{
					Essential:   true,
					Shell:       true,
					Instruction: "dnf install -y shadow-utils",
				},
			}
		case PackageManagerYum:
			return []Instruction{
				{
					Essential:   true,
					Shell:       true,
					Instruction: "yum install -y shadow-utils",
				},
			}
		default:
			return []Instruction{
				{
					Essential:   true,
					Shell:       false,
					Instruction: "newuidmap binary not found. Please install with a package manager.",
				},
			}
		}
	}
	return nil
}

func hasIptables() error {
	candidates := []string{
		"iptables",
		"/sbin/iptables",
		"/usr/sbin/iptables",
	}
	for _, cand := range candidates {
		if _, err := exec.LookPath(cand); err == nil {
			return nil
		}
	}
	return errors.New("iptables binary not found")
}

func hasIptablesMod() error {
	procModules, err := ioutil.ReadFile("/proc/modules")
	if err != nil {
		// ignore err
		return nil
	}
	if strings.Contains(string(procModules), "ip_tables") {
		// ip_tables is present as a loaded module
		return nil
	}
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		// ignore err
		return nil
	}
	modulesBuiltinPath := filepath.Join("/lib/modules", string(utsname.Release[:]), "modules.builtin")
	modulesBuiltin, err := ioutil.ReadFile(modulesBuiltinPath)
	if err != nil {
		// ignore err
		return nil
	}
	if strings.Contains(string(modulesBuiltin), "ip_tables") {
		// ip_tables is present as a built-in module
		return nil
	}
	return errors.New("ip_tables kernel module not found")
}

func instIptables() []Instruction {
	var insts []Instruction
	if err := hasIptables(); err != nil {
		switch getPackageManager() {
		case PackageManagerAptGet:
			insts = append(insts, Instruction{
				Essential:   !skipIptables,
				Shell:       true,
				Instruction: "apt-get install -y iptables",
			})
		case PackageManagerDnf:
			insts = append(insts, Instruction{
				Essential:   !skipIptables,
				Shell:       true,
				Instruction: "dnf install -y iptables",
			})
		case PackageManagerYum:
			insts = append(insts, Instruction{
				Essential:   !skipIptables,
				Shell:       true,
				Instruction: "yum install -y iptables",
			})
		default:
			insts = append(insts, Instruction{
				Essential:   !skipIptables,
				Shell:       false,
				Instruction: "iptables binary not found. Please install with a package manager.",
			})
		}
	}
	if err := hasIptablesMod(); err != nil {
		insts = append(insts, Instruction{
			Essential:   !skipIptables,
			Shell:       true,
			Instruction: "modprobe ip_tables",
		})
	}
	return insts
}

func instSysctl() []Instruction {
	var insts []Instruction
	// Debian
	if b, err := ioutil.ReadFile("/proc/sys/kernel/unprivileged_userns_clone"); err == nil {
		s := strings.TrimSpace(string(b))
		if s != "1" {
			insts = append(insts, Instruction{
				Essential: true,
				Shell:     true,
				Instruction: `cat <<EOT > /etc/sysctl.d/50-rootless.conf
kernel.unprivileged_userns_clone = 1
EOT
sysctl --system`,
			})
		}
	}
	// CentOS 7
	if b, err := ioutil.ReadFile("/proc/sys/user/max_user_namespaces"); err == nil {
		s := strings.TrimSpace(string(b))
		if s == "0" {
			insts = append(insts, Instruction{
				Essential: true,
				Shell:     true,
				Instruction: `cat <<EOT > /etc/sysctl.d/51-rootless.conf
user.max_user_namespaces = 28633
EOT
sysctl --system`,
			})
		}
	}
	return insts
}

func hasSubid(subgid bool) error {
	u, err := user.Current()
	if err != nil {
		// ignore err
		return nil
	}
	p := "/etc/subuid"
	if subgid {
		p = "/etc/subgid"
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	uidColon := u.Uid + ":"
	usernameColon := u.Username + ":"
	for scanner.Scan() {
		s := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(s, uidColon) || strings.HasPrefix(s, usernameColon) {
			return nil
		}
	}
	// ignore scanner.Err()
	return errors.Errorf("user %s not found in %s", u.Uid, p)
}

func instSubid() []Instruction {
	u, err := user.Current()
	if err != nil {
		// ignore err
		return nil
	}
	var insts []Instruction
	if err := hasSubid(false); err != nil {
		insts = append(insts, Instruction{
			Essential:   true,
			Shell:       true,
			Instruction: fmt.Sprintf("echo \"%s:100000:65536\" >> /etc/subuid", u.Username),
		})
	}
	if err := hasSubid(true); err != nil {
		insts = append(insts, Instruction{
			Essential:   true,
			Shell:       true,
			Instruction: fmt.Sprintf("echo \"%s:100000:65536\" >> /etc/subgid", u.Username),
		})
	}
	return insts
}

// TODO: add optional instructions for slirp4netns and fuse-overlayfs
