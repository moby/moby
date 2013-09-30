package docker

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
	"strconv"
	"strings"
)

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

func MergeConfig(userConf, imageConf *Config) {
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
	}

	if userConf.PortSpecs != nil && len(userConf.PortSpecs) > 0 {
		if userConf.ExposedPorts == nil {
			userConf.ExposedPorts = make(map[Port]struct{})
		}
		ports, _, err := parsePortSpecs(userConf.PortSpecs)
		if err != nil {
			panic(err)
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
			userConf.ExposedPorts = make(map[Port]struct{})
		}

		ports, _, err := parsePortSpecs(imageConf.PortSpecs)
		if err != nil {
			panic(err)
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
}

func parseLxcConfOpts(opts ListOpts) ([]KeyValuePair, error) {
	out := make([]KeyValuePair, len(opts))
	for i, o := range opts {
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

// We will receive port specs in the format of ip:public:private/proto and these need to be
// parsed in the internal types
func parsePortSpecs(ports []string) (map[Port]struct{}, map[Port][]PortBinding, error) {
	exposedPorts := make(map[Port]struct{}, len(ports))
	bindings := make(map[Port][]PortBinding)

	for _, rawPort := range ports {
		proto := "tcp"
		if i := strings.LastIndex(rawPort, "/"); i != -1 {
			proto = rawPort[i+1:]
			rawPort = rawPort[:i]
		}
		if !strings.Contains(rawPort, ":") {
			rawPort = fmt.Sprintf("::%s", rawPort)
		} else if len(strings.Split(rawPort, ":")) == 2 {
			rawPort = fmt.Sprintf(":%s", rawPort)
		}

		parts, err := utils.PartParser("ip:hostPort:containerPort", rawPort)
		if err != nil {
			return nil, nil, err
		}
		containerPort := parts["containerPort"]
		rawIp := parts["ip"]
		hostPort := parts["hostPort"]

		if containerPort == "" {
			return nil, nil, fmt.Errorf("No port specified: %s<empty>", rawPort)
		}

		port := NewPort(proto, containerPort)
		if _, exists := exposedPorts[port]; !exists {
			exposedPorts[port] = struct{}{}
		}

		binding := PortBinding{
			HostIp:   rawIp,
			HostPort: hostPort,
		}
		bslice, exists := bindings[port]
		if !exists {
			bslice = []PortBinding{}
		}
		bindings[port] = append(bslice, binding)
	}
	return exposedPorts, bindings, nil
}

// Splits a port in the format of port/proto
func splitProtoPort(rawPort string) (string, string) {
	parts := strings.Split(rawPort, "/")
	l := len(parts)
	if l == 0 {
		return "", ""
	}
	if l == 1 {
		return "tcp", rawPort
	}
	return parts[0], parts[1]
}

func parsePort(rawPort string) (int, error) {
	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		return 0, err
	}
	return int(port), nil
}

func migratePortMappings(config *Config) error {
	if config.PortSpecs != nil {
		// We don't have to worry about migrating the bindings to the host
		// This is our breaking change
		ports, _, err := parsePortSpecs(config.PortSpecs)
		if err != nil {
			return err
		}
		config.PortSpecs = nil

		if config.ExposedPorts == nil {
			config.ExposedPorts = make(map[Port]struct{}, len(ports))
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
