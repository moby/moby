package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/types"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v2"
)

// Moby is the type of a Moby config file
type Moby struct {
	Kernel struct {
		Image   string
		Cmdline string
	}
	Init     []string
	Onboot   []MobyImage
	Services []MobyImage
	Trust    TrustConfig
	Files    []struct {
		Path      string
		Directory bool
		Contents  string
	}
	Outputs []struct {
		Format  string
		Project string
		Bucket  string
		Family  string
		Keys    string
		Public  bool
		Replace bool
	}
}

// TrustConfig is the type of a content trust config
type TrustConfig struct {
	Image []string
	Org   []string
}

// MobyImage is the type of an image config
type MobyImage struct {
	Name              string
	Image             string
	Capabilities      []string
	Mounts            []specs.Mount
	Binds             []string
	Tmpfs             []string
	Command           []string
	Env               []string
	Cwd               string
	Net               string
	Pid               string
	Ipc               string
	Uts               string
	Readonly          bool
	MaskedPaths       []string `yaml:"maskedPaths"`
	ReadonlyPaths     []string `yaml:"readonlyPaths"`
	UID               uint32   `yaml:"uid"`
	GID               uint32   `yaml:"gid"`
	AdditionalGids    []uint32 `yaml:"additionalGids"`
	NoNewPrivileges   bool     `yaml:"noNewPrivileges"`
	Hostname          string
	OomScoreAdj       int    `yaml:"oomScoreAdj"`
	DisableOOMKiller  bool   `yaml:"disableOOMKiller"`
	RootfsPropagation string `yaml:"rootfsPropagation"`
	CgroupsPath       string `yaml:"cgroupsPath"`
	Sysctl            map[string]string
}

// recursively convert the map keys from `interface{}` to `string`
// this is needed to convert "yaml" interfaces to "JSON" interfaces
// see http://stackoverflow.com/questions/40737122/convert-yaml-to-json-without-struct-golang#answer-40737676
func convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k.(string)] = convert(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = convert(v)
		}
	}
	return i
}

// NewConfig parses a config file
func NewConfig(config []byte) (*Moby, error) {
	m := Moby{}

	// Parse raw yaml
	var rawYaml interface{}
	err := yaml.Unmarshal(config, &rawYaml)
	if err != nil {
		return &m, err
	}

	// Convert to raw JSON
	rawJSON := convert(rawYaml)

	// Validate raw yaml with JSON schema
	schemaLoader := gojsonschema.NewStringLoader(schema)
	documentLoader := gojsonschema.NewGoLoader(rawJSON)
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return &m, err
	}
	if !result.Valid() {
		fmt.Printf("The configuration file is invalid:\n")
		for _, desc := range result.Errors() {
			fmt.Printf("- %s\n", desc)
		}
		return &m, fmt.Errorf("invalid configuration file")
	}

	// Parse yaml
	err = yaml.Unmarshal(config, &m)
	if err != nil {
		return &m, err
	}

	return &m, nil
}

// ConfigToOCI converts a config specification to an OCI config file
func ConfigToOCI(image *MobyImage) ([]byte, error) {

	// TODO pass through same docker client to all functions
	cli, err := dockerClient()
	if err != nil {
		return []byte{}, err
	}

	inspect, err := dockerInspectImage(cli, image.Image)
	if err != nil {
		return []byte{}, err
	}

	return ConfigInspectToOCI(image, inspect)
}

func defaultMountpoint(tp string) string {
	switch tp {
	case "proc":
		return "/proc"
	case "devpts":
		return "/dev/pts"
	case "sysfs":
		return "/sys"
	case "cgroup":
		return "/sys/fs/cgroup"
	case "mqueue":
		return "/dev/mqueue"
	default:
		return ""
	}
}

// Sort mounts by number of path components so /dev/pts is listed after /dev
type mlist []specs.Mount

func (m mlist) Len() int {
	return len(m)
}
func (m mlist) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}
func (m mlist) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}
func (m mlist) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].Destination), string(os.PathSeparator))
}

// ConfigInspectToOCI converts a config and the output of image inspect to an OCI config file
func ConfigInspectToOCI(image *MobyImage, inspect types.ImageInspect) ([]byte, error) {
	oci := specs.Spec{}

	config := inspect.Config
	if config == nil {
		return []byte{}, errors.New("empty image config")
	}

	args := append(config.Entrypoint, config.Cmd...)
	if len(image.Command) != 0 {
		args = image.Command
	}
	env := config.Env
	if len(image.Env) != 0 {
		env = image.Env
	}
	cwd := config.WorkingDir
	if image.Cwd != "" {
		cwd = image.Cwd
	}
	if cwd == "" {
		cwd = "/"
	}
	// default options match what Docker does
	procOptions := []string{"nosuid", "nodev", "noexec", "relatime"}
	devOptions := []string{"nosuid", "strictatime", "mode=755", "size=65536k"}
	if image.Readonly {
		devOptions = append(devOptions, "ro")
	}
	ptsOptions := []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"}
	sysOptions := []string{"nosuid", "noexec", "nodev"}
	if image.Readonly {
		sysOptions = append(sysOptions, "ro")
	}
	cgroupOptions := []string{"nosuid", "noexec", "nodev", "relatime", "ro"}
	// note omits "standard" /dev/shm and /dev/mqueue
	mounts := map[string]specs.Mount{
		"/proc":          {Destination: "/proc", Type: "proc", Source: "proc", Options: procOptions},
		"/dev":           {Destination: "/dev", Type: "tmpfs", Source: "tmpfs", Options: devOptions},
		"/dev/pts":       {Destination: "/dev/pts", Type: "devpts", Source: "devpts", Options: ptsOptions},
		"/sys":           {Destination: "/sys", Type: "sysfs", Source: "sysfs", Options: sysOptions},
		"/sys/fs/cgroup": {Destination: "/sys/fs/cgroup", Type: "cgroup", Source: "cgroup", Options: cgroupOptions},
	}
	for _, t := range image.Tmpfs {
		parts := strings.Split(t, ":")
		if len(parts) > 2 {
			return []byte{}, fmt.Errorf("Cannot parse tmpfs, too many ':': %s", t)
		}
		dest := parts[0]
		opts := []string{}
		if len(parts) == 2 {
			opts = strings.Split(parts[2], ",")
		}
		mounts[dest] = specs.Mount{Destination: dest, Type: "tmpfs", Source: "tmpfs", Options: opts}
	}
	for _, b := range image.Binds {
		parts := strings.Split(b, ":")
		if len(parts) < 2 {
			return []byte{}, fmt.Errorf("Cannot parse bind, missing ':': %s", b)
		}
		if len(parts) > 3 {
			return []byte{}, fmt.Errorf("Cannot parse bind, too many ':': %s", b)
		}
		src := parts[0]
		dest := parts[1]
		opts := []string{"rw", "rbind", "rprivate"}
		if len(parts) == 3 {
			opts = strings.Split(parts[2], ",")
		}
		mounts[dest] = specs.Mount{Destination: dest, Type: "bind", Source: src, Options: opts}
	}
	for _, m := range image.Mounts {
		tp := m.Type
		src := m.Source
		dest := m.Destination
		opts := m.Options
		if tp == "" {
			switch src {
			case "mqueue", "devpts", "proc", "sysfs", "cgroup":
				tp = src
			}
		}
		if tp == "" && dest == "/dev" {
			tp = "tmpfs"
		}
		if tp == "" {
			return []byte{}, fmt.Errorf("Mount for destination %s is missing type", dest)
		}
		if src == "" {
			// usually sane, eg proc, tmpfs etc
			src = tp
		}
		if dest == "" {
			dest = defaultMountpoint(tp)
		}
		if dest == "" {
			return []byte{}, fmt.Errorf("Mount type %s is missing destination", tp)
		}
		mounts[dest] = specs.Mount{Destination: dest, Type: tp, Source: src, Options: opts}
	}
	mountList := mlist{}
	for _, m := range mounts {
		mountList = append(mountList, m)
	}
	sort.Sort(mountList)

	namespaces := []specs.LinuxNamespace{}
	// to attach to an existing namespace, easiest to bind mount with nsfs in a system container
	if image.Net != "host" {
		namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.NetworkNamespace, Path: image.Net})
	}
	if image.Pid != "host" {
		namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.PIDNamespace, Path: image.Pid})
	}
	if image.Ipc != "host" {
		namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.IPCNamespace, Path: image.Ipc})
	}
	if image.Uts != "host" {
		namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.UTSNamespace, Path: image.Uts})
	}
	// TODO user, cgroup namespaces, maybe mount=host if useful
	namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.MountNamespace})

	caps := image.Capabilities
	if len(caps) == 1 && strings.ToLower(caps[0]) == "all" {
		caps = []string{
			"CAP_AUDIT_CONTROL",
			"CAP_AUDIT_READ",
			"CAP_AUDIT_WRITE",
			"CAP_BLOCK_SUSPEND",
			"CAP_CHOWN",
			"CAP_DAC_OVERRIDE",
			"CAP_DAC_READ_SEARCH",
			"CAP_FOWNER",
			"CAP_FSETID",
			"CAP_IPC_LOCK",
			"CAP_IPC_OWNER",
			"CAP_KILL",
			"CAP_LEASE",
			"CAP_LINUX_IMMUTABLE",
			"CAP_MAC_ADMIN",
			"CAP_MAC_OVERRIDE",
			"CAP_MKNOD",
			"CAP_NET_ADMIN",
			"CAP_NET_BIND_SERVICE",
			"CAP_NET_BROADCAST",
			"CAP_NET_RAW",
			"CAP_SETFCAP",
			"CAP_SETGID",
			"CAP_SETPCAP",
			"CAP_SETUID",
			"CAP_SYSLOG",
			"CAP_SYS_ADMIN",
			"CAP_SYS_BOOT",
			"CAP_SYS_CHROOT",
			"CAP_SYS_MODULE",
			"CAP_SYS_NICE",
			"CAP_SYS_PACCT",
			"CAP_SYS_PTRACE",
			"CAP_SYS_RAWIO",
			"CAP_SYS_RESOURCE",
			"CAP_SYS_TIME",
			"CAP_SYS_TTY_CONFIG",
			"CAP_WAKE_ALARM",
		}
	}

	oci.Version = specs.Version

	oci.Platform = specs.Platform{
		OS:   inspect.Os,
		Arch: inspect.Architecture,
	}

	oci.Process = specs.Process{
		Terminal: false,
		//ConsoleSize
		User: specs.User{
			UID:            image.UID,
			GID:            image.GID,
			AdditionalGids: image.AdditionalGids,
			// Username (Windows)
		},
		Args: args,
		Env:  env,
		Cwd:  cwd,
		Capabilities: &specs.LinuxCapabilities{
			Bounding:    caps,
			Effective:   caps,
			Inheritable: caps,
			Permitted:   caps,
			Ambient:     []string{},
		},
		Rlimits:         []specs.LinuxRlimit{},
		NoNewPrivileges: image.NoNewPrivileges,
		// ApparmorProfile
		// SelinuxLabel
	}

	oci.Root = specs.Root{
		Path:     "rootfs",
		Readonly: image.Readonly,
	}

	oci.Hostname = image.Hostname
	oci.Mounts = mountList

	oci.Linux = &specs.Linux{
		// UIDMappings
		// GIDMappings
		Sysctl: image.Sysctl,
		Resources: &specs.LinuxResources{
			// Devices
			DisableOOMKiller: &image.DisableOOMKiller,
			// Memory
			// CPU
			// Pids
			// BlockIO
			// HugepageLimits
			// Network
		},
		CgroupsPath: image.CgroupsPath,
		Namespaces:  namespaces,
		// Devices
		// Seccomp
		RootfsPropagation: image.RootfsPropagation,
		MaskedPaths:       image.MaskedPaths,
		ReadonlyPaths:     image.ReadonlyPaths,
		// MountLabel
		// IntelRdt
	}

	return json.MarshalIndent(oci, "", "    ")
}

func filesystem(m *Moby) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	log.Infof("Add files:")
	for _, f := range m.Files {
		log.Infof("  %s", f.Path)
		if f.Path == "" {
			return buf, errors.New("Did not specify path for file")
		}
		if !f.Directory && f.Contents == "" {
			return buf, errors.New("Contents of file not specified")
		}
		// we need all the leading directories
		parts := strings.Split(path.Dir(f.Path), "/")
		root := ""
		for _, p := range parts {
			if p == "." || p == "/" {
				continue
			}
			if root == "" {
				root = p
			} else {
				root = root + "/" + p
			}
			hdr := &tar.Header{
				Name:     root,
				Typeflag: tar.TypeDir,
				Mode:     0700,
			}
			err := tw.WriteHeader(hdr)
			if err != nil {
				return buf, err
			}
		}

		if f.Directory {
			if f.Contents != "" {
				return buf, errors.New("Directory with contents not allowed")
			}
			hdr := &tar.Header{
				Name:     f.Path,
				Typeflag: tar.TypeDir,
				Mode:     0700,
			}
			err := tw.WriteHeader(hdr)
			if err != nil {
				return buf, err
			}
		} else {
			hdr := &tar.Header{
				Name: f.Path,
				Mode: 0600,
				Size: int64(len(f.Contents)),
			}
			err := tw.WriteHeader(hdr)
			if err != nil {
				return buf, err
			}
			_, err = tw.Write([]byte(f.Contents))
			if err != nil {
				return buf, err
			}
		}
	}
	return buf, nil
}
