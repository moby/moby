package service

import (
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/docker/volume"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-connections/nat"
	units "github.com/docker/go-units"
	"github.com/spf13/cobra"
)

var (
	// DefaultScale is the default scale to use for a replicated service
	DefaultScale uint64 = 1
)

type int64Value interface {
	Value() int64
}

type memBytes int64

func (m *memBytes) String() string {
	return strconv.FormatInt(m.Value(), 10)
}

func (m *memBytes) Set(value string) error {
	val, err := units.RAMInBytes(value)
	*m = memBytes(val)
	return err
}

func (m *memBytes) Type() string {
	return "MemoryBytes"
}

func (m *memBytes) Value() int64 {
	return int64(*m)
}

type nanoCPUs int64

func (c *nanoCPUs) String() string {
	return strconv.FormatInt(c.Value(), 10)
}

func (c *nanoCPUs) Set(value string) error {
	cpu, ok := new(big.Rat).SetString(value)
	if !ok {
		return fmt.Errorf("Failed to parse %v as a rational number", value)
	}
	*c = nanoCPUs(cpu.Mul(cpu, big.NewRat(1e9, 1)).Num().Int64())
	return nil
}

func (c *nanoCPUs) Type() string {
	return "NanoCPUs"
}

func (c *nanoCPUs) Value() int64 {
	return int64(*c)
}

// DurationOpt is an option type for time.Duration that uses a pointer. This
// allows us to get nil values outside, instead of defaulting to 0
type DurationOpt struct {
	value *time.Duration
}

// Set a new value on the option
func (d *DurationOpt) Set(s string) error {
	v, err := time.ParseDuration(s)
	d.value = &v
	return err
}

// Type returns the type of this option
func (d *DurationOpt) Type() string {
	return "duration-ptr"
}

// String returns a string repr of this option
func (d *DurationOpt) String() string {
	if d.value != nil {
		return d.value.String()
	}
	return "none"
}

// Value returns the time.Duration
func (d *DurationOpt) Value() *time.Duration {
	return d.value
}

// Uint64Opt represents a uint64.
type Uint64Opt struct {
	value *uint64
}

// Set a new value on the option
func (i *Uint64Opt) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	i.value = &v
	return err
}

// Type returns the type of this option
func (i *Uint64Opt) Type() string {
	return "uint64-ptr"
}

// String returns a string repr of this option
func (i *Uint64Opt) String() string {
	if i.value != nil {
		return fmt.Sprintf("%v", *i.value)
	}
	return "none"
}

// Value returns the time.Duration
func (i *Uint64Opt) Value() *uint64 {
	return i.value
}

type updateOptions struct {
	parallelism uint64
	delay       time.Duration
}

type resourceOptions struct {
	limitCPU      nanoCPUs
	limitMemBytes memBytes
	resCPU        nanoCPUs
	resMemBytes   memBytes
}

func (r *resourceOptions) ToResourceRequirements() *swarm.ResourceRequirements {
	return &swarm.ResourceRequirements{
		Limits: &swarm.Resources{
			NanoCPUs:    r.limitCPU.Value(),
			MemoryBytes: r.limitMemBytes.Value(),
		},
		Reservations: &swarm.Resources{
			NanoCPUs:    r.resCPU.Value(),
			MemoryBytes: r.resMemBytes.Value(),
		},
	}
}

type restartPolicyOptions struct {
	condition   string
	delay       DurationOpt
	maxAttempts Uint64Opt
	window      DurationOpt
}

func (r *restartPolicyOptions) ToRestartPolicy() *swarm.RestartPolicy {
	return &swarm.RestartPolicy{
		Condition:   swarm.RestartPolicyCondition(r.condition),
		Delay:       r.delay.Value(),
		MaxAttempts: r.maxAttempts.Value(),
		Window:      r.window.Value(),
	}
}

func convertNetworks(networks []string) []swarm.NetworkAttachmentConfig {
	nets := []swarm.NetworkAttachmentConfig{}
	for _, network := range networks {
		nets = append(nets, swarm.NetworkAttachmentConfig{Target: network})
	}
	return nets
}

type endpointOptions struct {
	mode    string
	ingress string
	ports   opts.ListOpts
}

func (e *endpointOptions) ToEndpointSpec() *swarm.EndpointSpec {
	portConfigs := []swarm.PortConfig{}
	// We can ignore errors because the format was already validated by ValidatePort
	ports, portBindings, _ := nat.ParsePortSpecs(e.ports.GetAll())

	for port := range ports {
		portConfigs = append(portConfigs, convertPortToPortConfig(port, portBindings)...)
	}

	return &swarm.EndpointSpec{
		Mode:         swarm.ResolutionMode(e.mode),
		Ingress:      swarm.IngressRouting(e.ingress),
		ExposedPorts: portConfigs,
	}
}

func convertPortToPortConfig(
	port nat.Port,
	portBindings map[nat.Port][]nat.PortBinding,
) []swarm.PortConfig {
	ports := []swarm.PortConfig{}

	for _, binding := range portBindings[port] {
		hostPort, _ := strconv.ParseUint(binding.HostPort, 10, 16)
		ports = append(ports, swarm.PortConfig{
			//TODO Name: ?
			Protocol:  swarm.PortConfigProtocol(port.Proto()),
			Port:      uint32(port.Int()),
			SwarmPort: uint32(hostPort),
		})
	}
	return ports
}

// ValidatePort validates a string is in the expected format for a port definition
func ValidatePort(value string) (string, error) {
	portMappings, err := nat.ParsePortSpec(value)
	for _, portMapping := range portMappings {
		if portMapping.Binding.HostIP != "" {
			return "", fmt.Errorf("HostIP is not supported by a service.")
		}
	}
	return value, err
}

// ValidateMount validates that a mount flag has the correct format
func ValidateMount(value string) (string, error) {
	// TODO: this is wrong when the client and daemon OS don't match
	_, err := volume.ParseMountSpec(value, "")
	return value, err
}

// ConvertMounts converts mount strings into a swarm.Mount object
func ConvertMounts(rawMounts []string) []swarm.Mount {
	mounts := []swarm.Mount{}

	for _, rawMount := range rawMounts {
		// TODO: this is wrong when the client and daemon OS don't match
		mountPoint, _ := volume.ParseMountSpec(rawMount, "")

		mounts = append(mounts, swarm.Mount{
			Target: mountPoint.Destination,
			Source: mountPoint.Source,
			// TODO: fix with new mounts
			//			Mask:       swarm.MountMask(mountPoint.Mode),
			//                      no more VolumeName
			Type: swarm.MountType(mountPoint.Type()),
		})
	}
	return mounts
}

type serviceOptions struct {
	name    string
	labels  opts.ListOpts
	image   string
	command []string
	args    []string
	env     opts.ListOpts
	workdir string
	user    string
	mounts  opts.ListOpts

	resources resourceOptions
	stopGrace DurationOpt

	scale Uint64Opt
	mode  string

	restartPolicy restartPolicyOptions
	constraints   []string
	update        updateOptions
	networks      []string
	endpoint      endpointOptions
}

func newServiceOptions() *serviceOptions {
	return &serviceOptions{
		labels: opts.NewListOpts(runconfigopts.ValidateEnv),
		env:    opts.NewListOpts(runconfigopts.ValidateEnv),
		mounts: opts.NewListOpts(ValidateMount),
		endpoint: endpointOptions{
			ports: opts.NewListOpts(ValidatePort),
		},
	}
}

func (opts *serviceOptions) ToService() swarm.ServiceSpec {
	service := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   opts.name,
			Labels: runconfigopts.ConvertKVStringsToMap(opts.labels.GetAll()),
		},
		TaskSpec: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:           opts.image,
				Command:         opts.command,
				Args:            opts.args,
				Env:             opts.env.GetAll(),
				Dir:             opts.workdir,
				User:            opts.user,
				Mounts:          ConvertMounts(opts.mounts.GetAll()),
				StopGracePeriod: opts.stopGrace.Value(),
			},
			Resources:     opts.resources.ToResourceRequirements(),
			RestartPolicy: opts.restartPolicy.ToRestartPolicy(),
			Placement: &swarm.Placement{
				Constraints: opts.constraints,
			},
		},
		Mode: swarm.ServiceMode{},
		UpdateConfig: &swarm.UpdateConfig{
			Parallelism: opts.update.parallelism,
			Delay:       opts.update.delay,
		},
		Networks:     convertNetworks(opts.networks),
		EndpointSpec: opts.endpoint.ToEndpointSpec(),
	}

	// TODO: add error if both global and instances are specified or if invalid value
	if opts.mode == "global" {
		service.Mode.Global = &swarm.GlobalService{}
	} else {
		service.Mode.Replicated = &swarm.ReplicatedService{
			Instances: opts.scale.Value(),
		}
	}
	return service
}

// addServiceFlags adds all flags that are common to both `create` and `update.
// Any flags that are not common are added separately in the individual command
func addServiceFlags(cmd *cobra.Command, opts *serviceOptions) {
	flags := cmd.Flags()
	flags.StringVar(&opts.name, "name", "", "Service name")
	flags.VarP(&opts.labels, "label", "l", "Service labels")

	flags.VarP(&opts.env, "env", "e", "Set environment variables")
	flags.StringVarP(&opts.workdir, "workdir", "w", "", "Working directory inside the container")
	flags.StringVarP(&opts.user, "user", "u", "", "Username or UID")
	flags.VarP(&opts.mounts, "volume", "v", "Attach a volume or create a bind mount")

	flags.Var(&opts.resources.limitCPU, "limit-cpu", "Limit CPUs")
	flags.Var(&opts.resources.limitMemBytes, "limit-memory", "Limit Memory")
	flags.Var(&opts.resources.resCPU, "reserve-cpu", "Reserve CPUs")
	flags.Var(&opts.resources.resMemBytes, "reserve-memory", "Reserve Memory")
	flags.Var(&opts.stopGrace, "stop-grace-period", "Time to wait before force killing a container")

	flags.StringVar(&opts.mode, "mode", "replicated", "Service mode (replicated or global)")
	flags.Var(&opts.scale, "scale", "Number of tasks")

	// TODO: help strings
	flags.StringVar(&opts.restartPolicy.condition, "restart-policy-condition", "", "")
	flags.Var(&opts.restartPolicy.delay, "restart-policy-delay", "")
	flags.Var(&opts.restartPolicy.maxAttempts, "restart-policy-max-attempts", "")
	flags.Var(&opts.restartPolicy.window, "restart-policy-window", "")

	flags.StringSliceVar(&opts.constraints, "constraint", []string{}, "Placement constraints")

	flags.Uint64Var(&opts.update.parallelism, "updateconfig-parallelism", 1, "UpdateConfig Parallelism")
	flags.DurationVar(&opts.update.delay, "updateconfig-delay", time.Duration(0), "UpdateConfig Delay")

	flags.StringSliceVar(&opts.networks, "network", []string{}, "Network attachments")
	flags.StringVar(&opts.endpoint.mode, "endpoint-mode", "", "Endpoint mode")
	flags.StringVar(&opts.endpoint.ingress, "endpoint-ingress", "", "Endpoint ingress")
	flags.VarP(&opts.endpoint.ports, "port", "p", "Publish a port as a node port")
}
