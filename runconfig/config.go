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
	SecurityOpt     []string
	Labels          map[string]string
}

func ContainerConfigFromJob(job *engine.Job) *Config {
	config := &Config{
		Hostname:        job.Getenv("Hostname"),
		Domainname:      job.Getenv("Domainname"),
		User:            job.Getenv("User"),
		Memory:          job.GetenvInt64("Memory"),
		MemorySwap:      job.GetenvInt64("MemorySwap"),
		CpuShares:       job.GetenvInt64("CpuShares"),
		Cpuset:          job.Getenv("Cpuset"),
		AttachStdin:     job.GetenvBool("AttachStdin"),
		AttachStdout:    job.GetenvBool("AttachStdout"),
		AttachStderr:    job.GetenvBool("AttachStderr"),
		Tty:             job.GetenvBool("Tty"),
		OpenStdin:       job.GetenvBool("OpenStdin"),
		StdinOnce:       job.GetenvBool("StdinOnce"),
		Image:           job.Getenv("Image"),
		WorkingDir:      job.Getenv("WorkingDir"),
		NetworkDisabled: job.GetenvBool("NetworkDisabled"),
		MacAddress:      job.Getenv("MacAddress"),
	}
	job.GetenvJson("ExposedPorts", &config.ExposedPorts)
	job.GetenvJson("Volumes", &config.Volumes)
	if PortSpecs := job.GetenvList("PortSpecs"); PortSpecs != nil {
		config.PortSpecs = PortSpecs
	}
	if Env := job.GetenvList("Env"); Env != nil {
		config.Env = Env
	}
	if Cmd := job.GetenvList("Cmd"); Cmd != nil {
		config.Cmd = Cmd
	}

	job.GetenvJson("Labels", &config.Labels)

	if Entrypoint := job.GetenvList("Entrypoint"); Entrypoint != nil {
		config.Entrypoint = Entrypoint
	}
	return config
}
