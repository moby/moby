package convert

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	composetypes "github.com/docker/docker/cli/compose/types"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/go-connections/nat"
)

// Services from compose-file types to engine API types
func Services(
	namespace Namespace,
	config *composetypes.Config,
) (map[string]swarm.ServiceSpec, error) {
	result := make(map[string]swarm.ServiceSpec)

	services := config.Services
	volumes := config.Volumes
	networks := config.Networks

	for _, service := range services {
		serviceSpec, err := convertService(namespace, service, networks, volumes)
		if err != nil {
			return nil, err
		}
		result[service.Name] = serviceSpec
	}

	return result, nil
}

func convertService(
	namespace Namespace,
	service composetypes.ServiceConfig,
	networkConfigs map[string]composetypes.NetworkConfig,
	volumes map[string]composetypes.VolumeConfig,
) (swarm.ServiceSpec, error) {
	name := namespace.Scope(service.Name)

	endpoint, err := convertEndpointSpec(service.Ports)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	mode, err := convertDeployMode(service.Deploy.Mode, service.Deploy.Replicas)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	mounts, err := Volumes(service.Volumes, volumes, namespace)
	if err != nil {
		// TODO: better error message (include service name)
		return swarm.ServiceSpec{}, err
	}

	resources, err := convertResources(service.Deploy.Resources)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	restartPolicy, err := convertRestartPolicy(
		service.Restart, service.Deploy.RestartPolicy)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	healthcheck, err := convertHealthcheck(service.HealthCheck)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	networks, err := convertServiceNetworks(service.Networks, networkConfigs, namespace, service.Name)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	var logDriver *swarm.Driver
	if service.Logging != nil {
		logDriver = &swarm.Driver{
			Name:    service.Logging.Driver,
			Options: service.Logging.Options,
		}
	}

	serviceSpec := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   name,
			Labels: AddStackLabel(namespace, service.Deploy.Labels),
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:           service.Image,
				Command:         service.Entrypoint,
				Args:            service.Command,
				Hostname:        service.Hostname,
				Hosts:           convertExtraHosts(service.ExtraHosts),
				Healthcheck:     healthcheck,
				Env:             convertEnvironment(service.Environment),
				Labels:          AddStackLabel(namespace, service.Labels),
				Dir:             service.WorkingDir,
				User:            service.User,
				Mounts:          mounts,
				StopGracePeriod: service.StopGracePeriod,
				TTY:             service.Tty,
				OpenStdin:       service.StdinOpen,
			},
			LogDriver:     logDriver,
			Resources:     resources,
			RestartPolicy: restartPolicy,
			Placement: &swarm.Placement{
				Constraints: service.Deploy.Placement.Constraints,
			},
		},
		EndpointSpec: endpoint,
		Mode:         mode,
		Networks:     networks,
		UpdateConfig: convertUpdateConfig(service.Deploy.UpdateConfig),
	}

	return serviceSpec, nil
}

func convertServiceNetworks(
	networks map[string]*composetypes.ServiceNetworkConfig,
	networkConfigs networkMap,
	namespace Namespace,
	name string,
) ([]swarm.NetworkAttachmentConfig, error) {
	if len(networks) == 0 {
		return []swarm.NetworkAttachmentConfig{
			{
				Target:  namespace.Scope("default"),
				Aliases: []string{name},
			},
		}, nil
	}

	nets := []swarm.NetworkAttachmentConfig{}
	for networkName, network := range networks {
		networkConfig, ok := networkConfigs[networkName]
		if !ok {
			return []swarm.NetworkAttachmentConfig{}, fmt.Errorf(
				"service %q references network %q, which is not declared", name, networkName)
		}
		var aliases []string
		if network != nil {
			aliases = network.Aliases
		}
		target := namespace.Scope(networkName)
		if networkConfig.External.External {
			target = networkConfig.External.Name
		}
		nets = append(nets, swarm.NetworkAttachmentConfig{
			Target:  target,
			Aliases: append(aliases, name),
		})
	}
	return nets, nil
}

func convertExtraHosts(extraHosts map[string]string) []string {
	hosts := []string{}
	for host, ip := range extraHosts {
		hosts = append(hosts, fmt.Sprintf("%s %s", ip, host))
	}
	return hosts
}

func convertHealthcheck(healthcheck *composetypes.HealthCheckConfig) (*container.HealthConfig, error) {
	if healthcheck == nil {
		return nil, nil
	}
	var (
		err               error
		timeout, interval time.Duration
		retries           int
	)
	if healthcheck.Disable {
		if len(healthcheck.Test) != 0 {
			return nil, fmt.Errorf("test and disable can't be set at the same time")
		}
		return &container.HealthConfig{
			Test: []string{"NONE"},
		}, nil

	}
	if healthcheck.Timeout != "" {
		timeout, err = time.ParseDuration(healthcheck.Timeout)
		if err != nil {
			return nil, err
		}
	}
	if healthcheck.Interval != "" {
		interval, err = time.ParseDuration(healthcheck.Interval)
		if err != nil {
			return nil, err
		}
	}
	if healthcheck.Retries != nil {
		retries = int(*healthcheck.Retries)
	}
	return &container.HealthConfig{
		Test:     healthcheck.Test,
		Timeout:  timeout,
		Interval: interval,
		Retries:  retries,
	}, nil
}

func convertRestartPolicy(restart string, source *composetypes.RestartPolicy) (*swarm.RestartPolicy, error) {
	// TODO: log if restart is being ignored
	if source == nil {
		policy, err := runconfigopts.ParseRestartPolicy(restart)
		if err != nil {
			return nil, err
		}
		switch {
		case policy.IsNone():
			return nil, nil
		case policy.IsAlways(), policy.IsUnlessStopped():
			return &swarm.RestartPolicy{
				Condition: swarm.RestartPolicyConditionAny,
			}, nil
		case policy.IsOnFailure():
			attempts := uint64(policy.MaximumRetryCount)
			return &swarm.RestartPolicy{
				Condition:   swarm.RestartPolicyConditionOnFailure,
				MaxAttempts: &attempts,
			}, nil
		default:
			return nil, fmt.Errorf("unknown restart policy: %s", restart)
		}
	}
	return &swarm.RestartPolicy{
		Condition:   swarm.RestartPolicyCondition(source.Condition),
		Delay:       source.Delay,
		MaxAttempts: source.MaxAttempts,
		Window:      source.Window,
	}, nil
}

func convertUpdateConfig(source *composetypes.UpdateConfig) *swarm.UpdateConfig {
	if source == nil {
		return nil
	}
	parallel := uint64(1)
	if source.Parallelism != nil {
		parallel = *source.Parallelism
	}
	return &swarm.UpdateConfig{
		Parallelism:     parallel,
		Delay:           source.Delay,
		FailureAction:   source.FailureAction,
		Monitor:         source.Monitor,
		MaxFailureRatio: source.MaxFailureRatio,
	}
}

func convertResources(source composetypes.Resources) (*swarm.ResourceRequirements, error) {
	resources := &swarm.ResourceRequirements{}
	if source.Limits != nil {
		cpus, err := opts.ParseCPUs(source.Limits.NanoCPUs)
		if err != nil {
			return nil, err
		}
		resources.Limits = &swarm.Resources{
			NanoCPUs:    cpus,
			MemoryBytes: int64(source.Limits.MemoryBytes),
		}
	}
	if source.Reservations != nil {
		cpus, err := opts.ParseCPUs(source.Reservations.NanoCPUs)
		if err != nil {
			return nil, err
		}
		resources.Reservations = &swarm.Resources{
			NanoCPUs:    cpus,
			MemoryBytes: int64(source.Reservations.MemoryBytes),
		}
	}
	return resources, nil
}

func convertEndpointSpec(source []string) (*swarm.EndpointSpec, error) {
	portConfigs := []swarm.PortConfig{}
	ports, portBindings, err := nat.ParsePortSpecs(source)
	if err != nil {
		return nil, err
	}

	for port := range ports {
		portConfigs = append(
			portConfigs,
			opts.ConvertPortToPortConfig(port, portBindings)...)
	}

	return &swarm.EndpointSpec{Ports: portConfigs}, nil
}

func convertEnvironment(source map[string]string) []string {
	var output []string

	for name, value := range source {
		output = append(output, fmt.Sprintf("%s=%s", name, value))
	}

	return output
}

func convertDeployMode(mode string, replicas *uint64) (swarm.ServiceMode, error) {
	serviceMode := swarm.ServiceMode{}

	switch mode {
	case "global":
		if replicas != nil {
			return serviceMode, fmt.Errorf("replicas can only be used with replicated mode")
		}
		serviceMode.Global = &swarm.GlobalService{}
	case "replicated", "":
		serviceMode.Replicated = &swarm.ReplicatedService{Replicas: replicas}
	default:
		return serviceMode, fmt.Errorf("Unknown mode: %s", mode)
	}
	return serviceMode, nil
}
