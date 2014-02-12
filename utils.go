package docker

import (
	"fmt"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/nat"
	"github.com/dotcloud/docker/pkg/namesgenerator"
	"github.com/dotcloud/docker/utils"
	"io"
	"strings"
	"sync/atomic"
)

type Change struct {
	archive.Change
}

// Compare two Config struct. Do not compare the "Image" nor "Hostname" fields
// If OpenStdin is set, then it differs
func CompareConfig(a, b *Config) bool {
	if a == nil || b == nil ||
		a.OpenStdin || b.OpenStdin {
		return false
	}
	if a.AttachStdout != b.AttachStdout ||
		a.AttachStderr != b.AttachStderr ||
		a.User != b.User ||
		a.Memory != b.Memory ||
		a.MemorySwap != b.MemorySwap ||
		a.CpuShares != b.CpuShares ||
		a.OpenStdin != b.OpenStdin ||
		a.Tty != b.Tty ||
		a.VolumesFrom != b.VolumesFrom {
		return false
	}
	if len(a.Cmd) != len(b.Cmd) ||
		len(a.Dns) != len(b.Dns) ||
		len(a.Env) != len(b.Env) ||
		len(a.PortSpecs) != len(b.PortSpecs) ||
		len(a.ExposedPorts) != len(b.ExposedPorts) ||
		len(a.Entrypoint) != len(b.Entrypoint) ||
		len(a.Volumes) != len(b.Volumes) {
		return false
	}

	for i := 0; i < len(a.Cmd); i++ {
		if a.Cmd[i] != b.Cmd[i] {
			return false
		}
	}
	for i := 0; i < len(a.Dns); i++ {
		if a.Dns[i] != b.Dns[i] {
			return false
		}
	}
	for i := 0; i < len(a.Env); i++ {
		if a.Env[i] != b.Env[i] {
			return false
		}
	}
	for i := 0; i < len(a.PortSpecs); i++ {
		if a.PortSpecs[i] != b.PortSpecs[i] {
			return false
		}
	}
	for k := range a.ExposedPorts {
		if _, exists := b.ExposedPorts[k]; !exists {
			return false
		}
	}
	for i := 0; i < len(a.Entrypoint); i++ {
		if a.Entrypoint[i] != b.Entrypoint[i] {
			return false
		}
	}
	for key := range a.Volumes {
		if _, exists := b.Volumes[key]; !exists {
			return false
		}
	}
	return true
}

func MergeConfig(userConf, imageConf *Config) error {
	if userConf.User == "" {
		userConf.User = imageConf.User
	}
	if userConf.Memory == 0 {
		userConf.Memory = imageConf.Memory
	}
	if userConf.MemorySwap == 0 {
		userConf.MemorySwap = imageConf.MemorySwap
	}
	if userConf.CpuShares == 0 {
		userConf.CpuShares = imageConf.CpuShares
	}
	if userConf.ExposedPorts == nil || len(userConf.ExposedPorts) == 0 {
		userConf.ExposedPorts = imageConf.ExposedPorts
	} else if imageConf.ExposedPorts != nil {
		if userConf.ExposedPorts == nil {
			userConf.ExposedPorts = make(nat.PortSet)
		}
		for port := range imageConf.ExposedPorts {
			if _, exists := userConf.ExposedPorts[port]; !exists {
				userConf.ExposedPorts[port] = struct{}{}
			}
		}
	}

	if userConf.PortSpecs != nil && len(userConf.PortSpecs) > 0 {
		if userConf.ExposedPorts == nil {
			userConf.ExposedPorts = make(nat.PortSet)
		}
		ports, _, err := nat.ParsePortSpecs(userConf.PortSpecs)
		if err != nil {
			return err
		}
		for port := range ports {
			if _, exists := userConf.ExposedPorts[port]; !exists {
				userConf.ExposedPorts[port] = struct{}{}
			}
		}
		userConf.PortSpecs = nil
	}
	if imageConf.PortSpecs != nil && len(imageConf.PortSpecs) > 0 {
		utils.Debugf("Migrating image port specs to containter: %s", strings.Join(imageConf.PortSpecs, ", "))
		if userConf.ExposedPorts == nil {
			userConf.ExposedPorts = make(nat.PortSet)
		}

		ports, _, err := nat.ParsePortSpecs(imageConf.PortSpecs)
		if err != nil {
			return err
		}
		for port := range ports {
			if _, exists := userConf.ExposedPorts[port]; !exists {
				userConf.ExposedPorts[port] = struct{}{}
			}
		}
	}
	if !userConf.Tty {
		userConf.Tty = imageConf.Tty
	}
	if !userConf.OpenStdin {
		userConf.OpenStdin = imageConf.OpenStdin
	}
	if !userConf.StdinOnce {
		userConf.StdinOnce = imageConf.StdinOnce
	}
	if userConf.Env == nil || len(userConf.Env) == 0 {
		userConf.Env = imageConf.Env
	} else {
		for _, imageEnv := range imageConf.Env {
			found := false
			imageEnvKey := strings.Split(imageEnv, "=")[0]
			for _, userEnv := range userConf.Env {
				userEnvKey := strings.Split(userEnv, "=")[0]
				if imageEnvKey == userEnvKey {
					found = true
				}
			}
			if !found {
				userConf.Env = append(userConf.Env, imageEnv)
			}
		}
	}
	if userConf.Cmd == nil || len(userConf.Cmd) == 0 {
		userConf.Cmd = imageConf.Cmd
	}
	if userConf.Dns == nil || len(userConf.Dns) == 0 {
		userConf.Dns = imageConf.Dns
	} else {
		//duplicates aren't an issue here
		userConf.Dns = append(userConf.Dns, imageConf.Dns...)
	}
	if userConf.Entrypoint == nil || len(userConf.Entrypoint) == 0 {
		userConf.Entrypoint = imageConf.Entrypoint
	}
	if userConf.WorkingDir == "" {
		userConf.WorkingDir = imageConf.WorkingDir
	}
	if userConf.VolumesFrom == "" {
		userConf.VolumesFrom = imageConf.VolumesFrom
	}
	if userConf.Volumes == nil || len(userConf.Volumes) == 0 {
		userConf.Volumes = imageConf.Volumes
	} else {
		for k, v := range imageConf.Volumes {
			userConf.Volumes[k] = v
		}
	}
	return nil
}

func parseLxcConfOpts(opts ListOpts) ([]KeyValuePair, error) {
	out := make([]KeyValuePair, opts.Len())
	for i, o := range opts.GetAll() {
		k, v, err := parseLxcOpt(o)
		if err != nil {
			return nil, err
		}
		out[i] = KeyValuePair{Key: k, Value: v}
	}
	return out, nil
}

func parseLxcOpt(opt string) (string, string, error) {
	parts := strings.SplitN(opt, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Unable to parse lxc conf option: %s", opt)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func migratePortMappings(config *Config, hostConfig *HostConfig) error {
	if config.PortSpecs != nil {
		ports, bindings, err := nat.ParsePortSpecs(config.PortSpecs)
		if err != nil {
			return err
		}
		config.PortSpecs = nil
		if len(bindings) > 0 {
			if hostConfig == nil {
				hostConfig = &HostConfig{}
			}
			hostConfig.PortBindings = bindings
		}

		if config.ExposedPorts == nil {
			config.ExposedPorts = make(nat.PortSet, len(ports))
		}
		for k, v := range ports {
			config.ExposedPorts[k] = v
		}
	}
	return nil
}

// Links come in the format of
// name:alias
func parseLink(rawLink string) (map[string]string, error) {
	return utils.PartParser("name:alias", rawLink)
}

type checker struct {
	runtime *Runtime
}

func (c *checker) Exists(name string) bool {
	return c.runtime.containerGraph.Exists("/" + name)
}

// Generate a random and unique name
func generateRandomName(runtime *Runtime) (string, error) {
	return namesgenerator.GenerateRandomName(&checker{runtime})
}

// Read an io.Reader and call a function when it returns EOF
func EofReader(r io.Reader, callback func()) *eofReader {
	return &eofReader{
		Reader:   r,
		callback: callback,
	}
}

type eofReader struct {
	io.Reader
	gotEOF   int32
	callback func()
}

func (r *eofReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err == io.EOF {
		// Use atomics to make the gotEOF check threadsafe
		if atomic.CompareAndSwapInt32(&r.gotEOF, 0, 1) {
			r.callback()
		}
	}
	return
}
