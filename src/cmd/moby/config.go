package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/types"
	"github.com/opencontainers/runtime-spec/specs-go"
	"gopkg.in/yaml.v2"
)

// Moby is the type of a Moby config file
type Moby struct {
	Kernel struct {
		Image   string
		Cmdline string
	}
	Init   string
	System []MobyImage
	Daemon []MobyImage
	Files  []struct {
		Path     string
		Contents string
	}
	Outputs []struct {
		Format  string
		Project string
		Bucket  string
		Family  string
		Public  bool
		Replace bool
	}
}

// MobyImage is the type of an image config
type MobyImage struct {
	Name             string
	Image            string
	Capabilities     []string
	Mounts           []specs.Mount
	Binds            []string
	Tmpfs            []string
	Command          []string
	Env              []string
	Cwd              string
	Net              string
	Pid              string
	Ipc              string
	Uts              string
	Readonly         bool
	UID              uint32   `yaml:"uid"`
	GID              uint32   `yaml:"gid"`
	AdditionalGids   []uint32 `yaml:"additionalGids"`
	NoNewPrivileges  bool     `yaml:"noNewPrivileges"`
	Hostname         string
	OomScoreAdj      int  `yaml:"oomScoreAdj"`
	DisableOOMKiller bool `yaml:"disableOOMKiller"`
}

// NewConfig parses a config file
func NewConfig(config []byte) (*Moby, error) {
	m := Moby{}

	err := yaml.Unmarshal(config, &m)
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
	mounts := []specs.Mount{
		{Destination: "/proc", Type: "proc", Source: "proc"},
		{Destination: "/dev", Type: "tmpfs", Source: "tmpfs", Options: devOptions},
		{Destination: "/dev/pts", Type: "devpts", Source: "devpts", Options: ptsOptions},
		{Destination: "/sys", Type: "sysfs", Source: "sysfs", Options: sysOptions},
		{Destination: "/sys/fs/cgroup", Type: "cgroup", Source: "cgroup", Options: cgroupOptions},
	}
	// TODO if any standard mount points supplied, remove from above, so can change options
	mounts = append(mounts, image.Mounts...)
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
		mounts = append(mounts, specs.Mount{Destination: dest, Type: "tmpfs", Source: "tmpfs", Options: opts})
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
		mounts = append(mounts, specs.Mount{Destination: dest, Type: "bind", Source: src, Options: opts})
	}

	namespaces := []specs.LinuxNamespace{}
	if image.Net != "" && image.Net != "host" {
		return []byte{}, fmt.Errorf("invalid net namespace: %s", image.Net)
	}
	if image.Net == "" {
		namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.NetworkNamespace})
	}
	if image.Pid != "" && image.Pid != "host" {
		return []byte{}, fmt.Errorf("invalid pid namespace: %s", image.Pid)
	}
	if image.Pid == "" {
		namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.PIDNamespace})
	}
	if image.Ipc != "" && image.Ipc != "host" {
		return []byte{}, fmt.Errorf("invalid ipc namespace: %s", image.Ipc)
	}
	if image.Ipc == "" {
		namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.IPCNamespace})
	}
	if image.Uts != "" && image.Uts != "host" {
		return []byte{}, fmt.Errorf("invalid uts namespace: %s", image.Uts)
	}
	if image.Uts == "" {
		namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.UTSNamespace})
	}
	// TODO user, cgroup namespaces, maybe mount=host if useful
	namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.MountNamespace})

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
			Bounding:    image.Capabilities,
			Effective:   image.Capabilities,
			Inheritable: image.Capabilities,
			Permitted:   image.Capabilities,
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
	oci.Mounts = mounts

	oci.Linux = &specs.Linux{
		// UIDMappings
		// GIDMappings
		// Sysctl
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
		// CgroupsPath
		Namespaces: namespaces,
		// Devices
		// Seccomp
		// RootfsPropagation
		// MaskedPaths
		// ReadonlyPaths
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
		if f.Contents == "" {
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
	return buf, nil
}
