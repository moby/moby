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
	Memory          int64  // Memory limit (in bytes)
	MemorySwap      int64  // Total memory usage (memory + swap); set `-1' to disable swap
	CpuShares       int64  // CPU shares (relative weight vs. other containers)
	Cpuset          string // Cpuset 0-2, 0,1
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
	OnBuild         []string
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
	if Entrypoint := job.GetenvList("Entrypoint"); Entrypoint != nil {
		config.Entrypoint = Entrypoint
	}
	return config
}
