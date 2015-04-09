package runconfig

import (
	"github.com/docker/docker/engine"
	"github.com/docker/docker/nat"
)

// Note: the Config structure should hold only portable information about the container.
// Here, "portable" means "independent from the host we are running on".
// Non-portable information *should* appear in HostConfig.
type Config struct {
	Hostname        string
	Domainname      string
	User            string
	Memory          int64  // FIXME: we keep it for backward compatibility, it has been moved to hostConfig.
	MemorySwap      int64  // FIXME: it has been moved to hostConfig.
	CpuShares       int64  // FIXME: it has been moved to hostConfig.
	Cpuset          string // FIXME: it has been moved to hostConfig and renamed to CpusetCpus.
	AttachStdin     bool
	AttachStdout    bool
	AttachStderr    bool
	PortSpecs       []string // Deprecated - Can be in the format of 8080/tcp
	ExposedPorts    map[nat.Port]struct{}
	Tty             bool // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin       bool // Open stdin
	StdinOnce       bool // If true, close stdin after the 1 attached client disconnects.
	Env             []string
	Cmd             []string
	Image           string // Name of the image as it was passed by the operator (eg. could be symbolic)
	Volumes         map[string]struct{}
	WorkingDir      string
	Entrypoint      []string
	NetworkDisabled bool
	MacAddress      string
	OnBuild         []string
	Labels          map[string]string
}

func ContainerConfigFromJob(env *engine.Env) *Config {
	config := &Config{
		Hostname:        env.Get("Hostname"),
		Domainname:      env.Get("Domainname"),
		User:            env.Get("User"),
		Memory:          env.GetInt64("Memory"),
		MemorySwap:      env.GetInt64("MemorySwap"),
		CpuShares:       env.GetInt64("CpuShares"),
		Cpuset:          env.Get("Cpuset"),
		AttachStdin:     env.GetBool("AttachStdin"),
		AttachStdout:    env.GetBool("AttachStdout"),
		AttachStderr:    env.GetBool("AttachStderr"),
		Tty:             env.GetBool("Tty"),
		OpenStdin:       env.GetBool("OpenStdin"),
		StdinOnce:       env.GetBool("StdinOnce"),
		Image:           env.Get("Image"),
		WorkingDir:      env.Get("WorkingDir"),
		NetworkDisabled: env.GetBool("NetworkDisabled"),
		MacAddress:      env.Get("MacAddress"),
	}
	env.GetJson("ExposedPorts", &config.ExposedPorts)
	env.GetJson("Volumes", &config.Volumes)
	if PortSpecs := env.GetList("PortSpecs"); PortSpecs != nil {
		config.PortSpecs = PortSpecs
	}
	if Env := env.GetList("Env"); Env != nil {
		config.Env = Env
	}
	if Cmd := env.GetList("Cmd"); Cmd != nil {
		config.Cmd = Cmd
	}

	env.GetJson("Labels", &config.Labels)

	if Entrypoint := env.GetList("Entrypoint"); Entrypoint != nil {
		config.Entrypoint = Entrypoint
	}
	return config
}
