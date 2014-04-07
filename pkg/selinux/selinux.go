package selinux

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/dotcloud/docker/pkg/mount"
	"github.com/dotcloud/docker/pkg/system"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

const (
	Enforcing        = 1
	Permissive       = 0
	Disabled         = -1
	selinuxDir       = "/etc/selinux/"
	selinuxConfig    = selinuxDir + "config"
	selinuxTypeTag   = "SELINUXTYPE"
	selinuxTag       = "SELINUX"
	selinuxPath      = "/sys/fs/selinux"
	xattrNameSelinux = "security.selinux"
	stRdOnly         = 0x01
)

var (
	assignRegex           = regexp.MustCompile(`^([^=]+)=(.*)$`)
	spaceRegex            = regexp.MustCompile(`^([^=]+) (.*)$`)
	mcsList               = make(map[string]bool)
	selinuxfs             = "unknown"
	selinuxEnabled        = false
	selinuxEnabledChecked = false
)

type SELinuxContext map[string]string

// SetDisabled disables selinux support for the package
func SetDisabled() {
	selinuxEnabled, selinuxEnabledChecked = false, true
}

func getSelinuxMountPoint() string {
	if selinuxfs != "unknown" {
		return selinuxfs
	}
	selinuxfs = ""

	mounts, err := mount.GetMounts()
	if err != nil {
		return selinuxfs
	}
	for _, mount := range mounts {
		if mount.Fstype == "selinuxfs" {
			selinuxfs = mount.Mountpoint
			break
		}
	}
	if selinuxfs != "" {
		var buf syscall.Statfs_t
		syscall.Statfs(selinuxfs, &buf)
		if (buf.Flags & stRdOnly) == 1 {
			selinuxfs = ""
		}
	}
	return selinuxfs
}

func SelinuxEnabled() bool {
	if selinuxEnabledChecked {
		return selinuxEnabled
	}
	selinuxEnabledChecked = true
	if fs := getSelinuxMountPoint(); fs != "" {
		if con, _ := getcon(); con != "kernel" {
			selinuxEnabled = true
		}
	}
	return selinuxEnabled
}

func readConfig(target string) (value string) {
	var (
		val, key string
		bufin    *bufio.Reader
	)

	in, err := os.Open(selinuxConfig)
	if err != nil {
		return ""
	}
	defer in.Close()

	bufin = bufio.NewReader(in)

	for done := false; !done; {
		var line string
		if line, err = bufin.ReadString('\n'); err != nil {
			if err != io.EOF {
				return ""
			}
			done = true
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Skip blank lines
			continue
		}
		if line[0] == ';' || line[0] == '#' {
			// Skip comments
			continue
		}
		if groups := assignRegex.FindStringSubmatch(line); groups != nil {
			key, val = strings.TrimSpace(groups[1]), strings.TrimSpace(groups[2])
			if key == target {
				return strings.Trim(val, "\"")
			}
		}
	}
	return ""
}

func getSELinuxPolicyRoot() string {
	return selinuxDir + readConfig(selinuxTypeTag)
}

func readCon(name string) (string, error) {
	var val string

	in, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer in.Close()

	_, err = fmt.Fscanf(in, "%s", &val)
	return val, err
}

func Setfilecon(path string, scon string) error {
	return system.Lsetxattr(path, xattrNameSelinux, []byte(scon), 0)
}

func Setfscreatecon(scon string) error {
	return writeCon("/proc/self/attr/fscreate", scon)
}

func Getfscreatecon() (string, error) {
	return readCon("/proc/self/attr/fscreate")
}

func getcon() (string, error) {
	return readCon("/proc/self/attr/current")
}

func Getpidcon(pid int) (string, error) {
	return readCon(fmt.Sprintf("/proc/%d/attr/current", pid))
}

func Getexeccon() (string, error) {
	return readCon("/proc/self/attr/exec")
}

func writeCon(name string, val string) error {
	if !SelinuxEnabled() {
		return nil
	}
	out, err := os.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer out.Close()

	if val != "" {
		_, err = out.Write([]byte(val))
	} else {
		_, err = out.Write(nil)
	}
	return err
}

func Setexeccon(scon string) error {
	return writeCon(fmt.Sprintf("/proc/self/task/%d/attr/exec", system.Gettid()), scon)
}

func (c SELinuxContext) Get() string {
	return fmt.Sprintf("%s:%s:%s:%s", c["user"], c["role"], c["type"], c["level"])
}

func NewContext(scon string) SELinuxContext {
	c := make(SELinuxContext)

	if len(scon) != 0 {
		con := strings.SplitN(scon, ":", 4)
		c["user"] = con[0]
		c["role"] = con[1]
		c["type"] = con[2]
		c["level"] = con[3]
	}
	return c
}

func SelinuxGetEnforce() int {
	var enforce int

	enforceS, err := readCon(fmt.Sprintf("%s/enforce", selinuxPath))
	if err != nil {
		return -1
	}

	enforce, err = strconv.Atoi(string(enforceS))
	if err != nil {
		return -1
	}
	return enforce
}

func SelinuxGetEnforceMode() int {
	switch readConfig(selinuxTag) {
	case "enforcing":
		return Enforcing
	case "permissive":
		return Permissive
	}
	return Disabled
}

func mcsAdd(mcs string) {
	mcsList[mcs] = true
}

func mcsDelete(mcs string) {
	mcsList[mcs] = false
}

func mcsExists(mcs string) bool {
	return mcsList[mcs]
}

func IntToMcs(id int, catRange uint32) string {
	var (
		SETSIZE = int(catRange)
		TIER    = SETSIZE
		ORD     = id
	)

	if id < 1 || id > 523776 {
		return ""
	}

	for ORD > TIER {
		ORD = ORD - TIER
		TIER -= 1
	}
	TIER = SETSIZE - TIER
	ORD = ORD + TIER
	return fmt.Sprintf("s0:c%d,c%d", TIER, ORD)
}

func uniqMcs(catRange uint32) string {
	var (
		n      uint32
		c1, c2 uint32
		mcs    string
	)

	for {
		binary.Read(rand.Reader, binary.LittleEndian, &n)
		c1 = n % catRange
		binary.Read(rand.Reader, binary.LittleEndian, &n)
		c2 = n % catRange
		if c1 == c2 {
			continue
		} else {
			if c1 > c2 {
				t := c1
				c1 = c2
				c2 = t
			}
		}
		mcs = fmt.Sprintf("s0:c%d,c%d", c1, c2)
		if mcsExists(mcs) {
			continue
		}
		mcsAdd(mcs)
		break
	}
	return mcs
}

func GetLxcContexts() (processLabel string, fileLabel string) {
	var (
		val, key string
		bufin    *bufio.Reader
	)

	if !SelinuxEnabled() {
		return "", ""
	}
	lxcPath := fmt.Sprintf("%s/contexts/lxc_contexts", getSELinuxPolicyRoot())
	in, err := os.Open(lxcPath)
	if err != nil {
		return "", ""
	}
	defer in.Close()

	bufin = bufio.NewReader(in)

	for done := false; !done; {
		var line string
		if line, err = bufin.ReadString('\n'); err != nil {
			if err == io.EOF {
				done = true
			} else {
				goto exit
			}
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Skip blank lines
			continue
		}
		if line[0] == ';' || line[0] == '#' {
			// Skip comments
			continue
		}
		if groups := assignRegex.FindStringSubmatch(line); groups != nil {
			key, val = strings.TrimSpace(groups[1]), strings.TrimSpace(groups[2])
			if key == "process" {
				processLabel = strings.Trim(val, "\"")
			}
			if key == "file" {
				fileLabel = strings.Trim(val, "\"")
			}
		}
	}

	if processLabel == "" || fileLabel == "" {
		return "", ""
	}

exit:
	mcs := IntToMcs(os.Getpid(), 1024)
	scon := NewContext(processLabel)
	scon["level"] = mcs
	processLabel = scon.Get()
	scon = NewContext(fileLabel)
	scon["level"] = mcs
	fileLabel = scon.Get()
	return processLabel, fileLabel
}

func SecurityCheckContext(val string) error {
	return writeCon(fmt.Sprintf("%s.context", selinuxPath), val)
}

func CopyLevel(src, dest string) (string, error) {
	if !SelinuxEnabled() {
		return "", nil
	}
	if src == "" {
		return "", nil
	}
	if err := SecurityCheckContext(src); err != nil {
		return "", err
	}
	if err := SecurityCheckContext(dest); err != nil {
		return "", err
	}
	scon := NewContext(src)
	tcon := NewContext(dest)
	tcon["level"] = scon["level"]
	return tcon.Get(), nil
}
