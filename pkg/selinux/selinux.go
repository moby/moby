package selinux

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var (
	SELINUXDIR         = "/etc/selinux/"
	SELINUXCONFIG      = SELINUXDIR + "config"
	SELINUXTYPETAG     = "SELINUXTYPE"
	SELINUXTAG         = "SELINUX"
	SELINUX_PATH       = "/sys/fs/selinux"
	XATTR_NAME_SELINUX = "security.selinux"
	assignRegex        = regexp.MustCompile(`^([^=]+)=(.*)$`)
	spaceRegex         = regexp.MustCompile(`^([^=]+) (.*)$`)
	mcsList            = make(map[string]bool)
	selinuxfs          = "unknown"
	ST_RDONLY          = int64(1)
)

const (
	Enforcing  = 1
	Permissive = 0
	Disabled   = -1
)

func GetSelinuxFileSystemPath() string {
	var bufin *bufio.Reader

	if selinuxfs != "unknown" {
		return selinuxfs
	}
	selinuxfs = ""

	in, err := os.Open("/proc/mounts")
	if err != nil {
		return ""
	}

	defer in.Close()
	bufin = bufio.NewReader(in)

	for done := false; !done; {
		var line string
		if line, err = bufin.ReadString('\n'); err != nil {
			if err == io.EOF {
				done = true
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
		groups := strings.Split(line, " ")
		if groups[0] == "selinuxfs" {
			selinuxfs = strings.Trim(groups[1], "\"")
			done = true
		}
	}
	var buf syscall.Statfs_t
	syscall.Statfs(selinuxfs, &buf)
	if (buf.Flags & ST_RDONLY) == 1 {
		selinuxfs = ""
	}
	return selinuxfs
}

func SelinuxEnabled() bool {
	fs := GetSelinuxFileSystemPath()
	if fs == "" {
		return false
	}
	con, _ := Getcon()
	if con == "kernel" {
		return false
	}
	return true
}

func ReadConfig(target string) (value string) {
	var val, key string
	var bufin *bufio.Reader

	in, err := os.Open(SELINUXCONFIG)
	if err != nil {
		return ""
	}

	defer in.Close()
	bufin = bufio.NewReader(in)

	for done := false; !done; {
		var line string
		if line, err = bufin.ReadString('\n'); err != nil {
			if err == io.EOF {
				done = true
			} else {
				return ""
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
			if key == target {
				return strings.Trim(val, "\"")
			}
		}
	}
	return ""
}

func GetSELinuxPolicyRoot() string {
	return SELINUXDIR + ReadConfig(SELINUXTYPETAG)
}

func readCon(name string) (string, error) {
	var val string

	in, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := in.Close(); err != nil {
			panic(err)
		}
	}()
	_, err = fmt.Fscanf(in, "%s", &val)
	return val, err
}

func Setfilecon(path string, scon string) error {
	return syscall.Setxattr(path, XATTR_NAME_SELINUX, []byte(scon), 0)
}

func Getfilecon(path string) (string, error) {
	var scon []byte
	cnt, err := syscall.Getxattr(path, XATTR_NAME_SELINUX, scon)
	scon = make([]byte, cnt)
	cnt, err = syscall.Getxattr(path, XATTR_NAME_SELINUX, scon)
	return string(scon), err
}

func Setfscreatecon(scon string) error {
	return writeCon("/proc/self/attr/fscreate", scon)
}

func Getfscreatecon() (string, error) {
	return readCon("/proc/self/attr/fscreate")
}

func Getcon() (string, error) {
	return readCon("/proc/self/attr/current")
}

func Getpidcon(pid int) (string, error) {
	return readCon(fmt.Sprintf("/proc/%d/attr/current", pid))
}

func writeCon(name string, val string) error {
	if !SelinuxEnabled() {
		return nil
	}
	out, err := os.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			panic(err)
		}
	}()

	if val != "" {
		_, err = out.Write([]byte(val))
	} else {
		_, err = out.Write(nil)
	}
	return err
}

func Setexeccon(scon string) error {
	return writeCon("/proc/self/attr/exec", scon)
}

type Context struct {
	con []string
}

func (c *Context) SetUser(user string) {
	c.con[0] = user
}
func (c *Context) GetUser() string {
	return c.con[0]
}
func (c *Context) SetRole(role string) {
	c.con[1] = role
}
func (c *Context) GetRole() string {
	return c.con[1]
}
func (c *Context) SetType(setype string) {
	c.con[2] = setype
}
func (c *Context) GetType() string {
	return c.con[2]
}
func (c *Context) SetLevel(mls string) {
	c.con[3] = mls
}
func (c *Context) GetLevel() string {
	return c.con[3]
}
func (c *Context) Get() string {
	return strings.Join(c.con, ":")
}
func (c *Context) Set(scon string) {
	c.con = strings.SplitN(scon, ":", 4)
}
func NewContext(scon string) Context {
	var con Context
	con.Set(scon)
	return con
}

func SelinuxGetEnforce() int {
	var enforce int
	enforceS, err := readCon(fmt.Sprintf("%s/enforce", SELINUX_PATH))
	if err != nil {
		return -1
	}

	enforce, err = strconv.Atoi("1")
	enforce, err = strconv.Atoi(string(enforceS))
	if err != nil {
		return -1
	}
	return enforce
}

func SelinuxGetEnforceMode() int {
	switch ReadConfig(SELINUXTAG) {
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
	if (id < 1) || (id > 523776) {
		return ""
	}

	SETSIZE := int(catRange)
	TIER := SETSIZE

	ORD := id
	for ORD > TIER {
		ORD = ORD - TIER
		TIER -= 1
	}
	TIER = SETSIZE - TIER
	ORD = ORD + TIER
	return fmt.Sprintf("s0:c%d,c%d", TIER, ORD)
}

func uniqMcs(catRange uint32) string {
	var n uint32
	var c1, c2 uint32
	var mcs string
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
func freeContext(con string) {
	var scon Context
	if con != "" {
		scon = NewContext(con)
		mcsDelete(scon.GetLevel())
	}
}

func GetLxcContexts() (processLabel string, fileLabel string) {
	var val, key string
	var bufin *bufio.Reader
	if !SelinuxEnabled() {
		return "", ""
	}
	lxcPath := fmt.Sprintf("%s/content/lxc_contexts", GetSELinuxPolicyRoot())
	fileLabel = "system_u:object_r:svirt_sandbox_file_t:s0"
	processLabel = "system_u:system_r:svirt_lxc_net_t:s0"

	in, err := os.Open(lxcPath)
	if err != nil {
		goto exit
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
exit:
	var scon Context
	mcs := IntToMcs(os.Getpid(), 1024)
	scon = NewContext(processLabel)
	scon.SetLevel(mcs)
	processLabel = scon.Get()
	scon = NewContext(fileLabel)
	scon.SetLevel(mcs)
	fileLabel = scon.Get()
	return processLabel, fileLabel
}
func Security_check_context(val string) error {
	return writeCon(fmt.Sprintf("%s.context", SELINUX_PATH), val)
}

func CopyLevel(src, dest string) (string, error) {
	if !SelinuxEnabled() {
		return "", nil
	}
	if src == "" {
		return "", nil
	}
	err := Security_check_context(src)
	if err != nil {
		return "", err
	}
	err = Security_check_context(dest)
	if err != nil {
		return "", err
	}
	scon := NewContext(src)
	tcon := NewContext(dest)
	tcon.SetLevel(scon.GetLevel())
	return tcon.Get(), nil
}

func Test() {
	var err error
	var plabel, flabel string
	if SelinuxEnabled() {
		fmt.Println("Enabled")
	} else {
		fmt.Println("Disabled")
	}

	plabel, flabel = GetLxcContexts()
	fmt.Println(plabel)
	fmt.Println(flabel)
	freeContext(plabel)
	plabel, flabel = GetLxcContexts()
	fmt.Println(plabel)
	fmt.Println(flabel)
	freeContext(plabel)
	fmt.Println("getenforce ", SelinuxGetEnforce())
	fmt.Println("getenforcemode ", SelinuxGetEnforceMode())
	pid := os.Getpid()
	fmt.Printf("PID:%d MCS:%s\n", pid, IntToMcs(pid, 1023))
	fmt.Println(Getcon())
	fmt.Println(Getfilecon("/etc/passwd"))
	err = Setfscreatecon("unconfined_u:unconfined_r:unconfined_t:s0")
	if err == nil {
		fmt.Println(Getfscreatecon())
	} else {
		fmt.Println("setfscreatecon failed", err)
	}
	err = Setfscreatecon("")
	if err == nil {
		fmt.Println(Getfscreatecon())
	} else {
		fmt.Println("setfscreatecon failed", err)
	}
	fmt.Println(Getpidcon(1))
	err = Setfilecon("/home/dwalsh/.emacs", "staff_u:object_r:home_bin_t:s0")
	if err == nil {
		fmt.Println(Getfilecon("/home/dwalsh/.emacs"))
	} else {
		fmt.Println("Setfilecon failed")
	}
	fmt.Println(GetSelinuxFileSystemPath())
}
