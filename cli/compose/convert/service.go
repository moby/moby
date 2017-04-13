package convert

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/versions"
	servicecli "github.com/docker/docker/cli/command/service"
	composetypes "github.com/docker/docker/cli/compose/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/pkg/errors"
)

const defaultNetwork = "default"

// Services from compose-file types to engine API types
func Services(
	namespace Namespace,
	config *composetypes.Config,
	client client.CommonAPIClient,
) (map[string]swarm.ServiceSpec, error) {
	result := make(map[string]swarm.ServiceSpec)

	services := config.Services
	volumes := config.Volumes
	networks := config.Networks

	for _, service := range services {
		secrets, err := convertServiceSecrets(client, namespace, service.Secrets, config.Secrets)
		if err != nil {
			return nil, errors.Wrapf(err, "service %s", service.Name)
		}
		serviceSpec, err := convertService(client.ClientVersion(), namespace, service, networks, volumes, secrets)
		if err != nil {
			return nil, errors.Wrapf(err, "service %s", service.Name)
		}
		result[service.Name] = serviceSpec
	}

	return result, nil
}

func convertService(
	apiVersion string,
	namespace Namespace,
	service composetypes.ServiceConfig,
	networkConfigs map[string]composetypes.NetworkConfig,
	volumes map[string]composetypes.VolumeConfig,
	secrets []*swarm.SecretReference,
) (swarm.ServiceSpec, error) {
	name := namespace.Scope(service.Name)

	endpoint, err := convertEndpointSpec(service.Deploy.EndpointMode, service.Ports)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	mode, err := convertDeployMode(service.Deploy.Mode, service.Deploy.Replicas)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	mounts, err := Volumes(service.Volumes, volumes, namespace)
	if err != nil {
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

	dnsConfig, err := convertDNSConfig(service.DNS, service.DNSSearch)
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
				Hosts:           sortStrings(convertExtraHosts(service.ExtraHosts)),
				DNSConfig:       dnsConfig,
				Healthcheck:     healthcheck,
				Env:             sortStrings(convertEnvironment(service.Environment)),
				Labels:          AddStackLabel(namespace, service.Labels),
				Dir:             service.WorkingDir,
				User:            service.User,
				Mounts:          mounts,
				StopGracePeriod: service.StopGracePeriod,
				TTY:             service.Tty,
				OpenStdin:       service.StdinOpen,
				Secrets:         secrets,
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
		UpdateConfig: convertUpdateConfig(service.Deploy.UpdateConfig),
	}

	// ServiceSpec.Networks is deprecated and should not have been used by
	// this package. It is possible to update TaskTemplate.Networks, but it
	// is not possible to update ServiceSpec.Networks. Unfortunately, we
	// can't unconditionally start using TaskTemplate.Networks, because that
	// will break with older daemons that don't support migrating from
	// ServiceSpec.Networks to TaskTemplate.Networks. So which field to use
	// is conditional on daemon version.
	if versions.LessThan(apiVersion, "1.29") {
		serviceSpec.Networks = networks
	} else {
		serviceSpec.TaskTemplate.Networks = networks
	}
	return serviceSpec, nil
}

func sortStrings(strs []string) []string {
	sort.Strings(strs)
	return strs
}

type byNetworkTarget []swarm.NetworkAttachmentConfig

func (a byNetworkTarget) Len() int           { return len(a) }
func (a byNetworkTarget) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byNetworkTarget) Less(i, j int) bool { return a[i].Target < a[j].Target }

func convertServiceNetworks(
	networks map[string]*composetypes.ServiceNetworkConfig,
	networkConfigs networkMap,
	namespace Namespace,
	name string,
) ([]swarm.NetworkAttachmentConfig, error) {
	if len(networks) == 0 {
		networks = map[string]*composetypes.ServiceNetworkConfig{
			defaultNetwork: {},
		}
	}

	nets := []swarm.NetworkAttachmentConfig{}
	for networkName, network := range networks {
		networkConfig, ok := networkConfigs[networkName]
		if !ok && networkName != defaultNetwork {
			return nil, errors.Errorf("undefined network %q", networkName)
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

	sort.Sort(byNetworkTarget(nets))
	return nets, nil
}

// TODO: fix secrets API so that SecretAPIClient is not required here
func convertServiceSecrets(
	client client.SecretAPIClient,
	namespace Namespace,
	secrets []composetypes.ServiceSecretConfig,
	secretSpecs map[string]composetypes.SecretConfig,
) ([]*swarm.SecretReference, error) {
	refs := []*swarm.SecretReference{}
	for _, secret := range secrets {
		target := secret.Target
		if target == "" {
			target = secret.Source
		}

		secretSpec, exists := secretSpecs[secret.Source]
		if !exists {
			return nil, errors.Errorf("undefined secret %q", secret.Source)
		}

		source := namespace.Scope(secret.Source)
		if secretSpec.External.External {
			source = secretSpec.External.Name
		}

		uid := secret.UID
		gid := secret.GID
		if uid == "" {
			uid = "0"
		}
		if gid == "" {
			gid = "0"
		}
		mode := secret.Mode
		if mode == nil {
			mode = uint32Ptr(0444)
		}

		refs = append(refs, &swarm.SecretReference{
			File: &swarm.SecretReferenceFileTarget{
				Name: target,
				UID:  uid,
				GID:  gid,
				Mode: os.FileMode(*mode),
			},
			SecretName: source,
		})
	}

	return servicecli.ParseSecrets(client, refs)
}

func uint32Ptr(value uint32) *uint32 {
	return &value
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
		err                            error
		timeout, interval, startPeriod time.Duration
		retries                        int
	)
	if healthcheck.Disable {
		if len(healthcheck.Test) != 0 {
			return nil, errors.Errorf("test and disable can't be set at the same time")
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
	if healthcheck.StartPeriod != "" {
		startPeriod, err = time.ParseDuration(healthcheck.StartPeriod)
		if err != nil {
			return nil, err
		}
	}
	if healthcheck.Retries != nil {
		retries = int(*healthcheck.Retries)
	}
	return &container.HealthConfig{
		Test:        healthcheck.Test,
		Timeout:     timeout,
		Interval:    interval,
		Retries:     retries,
		StartPeriod: startPeriod,
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
			return nil, errors.Errorf("unknown restart policy: %s", restart)
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
	var err error
	if source.Limits != nil {
		var cpus int64
		if source.Limits.NanoCPUs != "" {
			cpus, err = opts.ParseCPUs(source.Limits.NanoCPUs)
			if err != nil {
				return nil, err
			}
		}
		resources.Limits = &swarm.Resources{
			NanoCPUs:    cpus,
			MemoryBytes: int64(source.Limits.MemoryBytes),
		}
	}
	if source.Reservations != nil {
		var cpus int64
		if source.Reservations.NanoCPUs != "" {
			cpus, err = opts.ParseCPUs(source.Reservations.NanoCPUs)
			if err != nil {
				return nil, err
			}
		}
		resources.Reservations = &swarm.Resources{
			NanoCPUs:    cpus,
			MemoryBytes: int64(source.Reservations.MemoryBytes),
		}
	}
	return resources, nil
}

type byPublishedPort []swarm.PortConfig

func (a byPublishedPort) Len() int           { return len(a) }
func (a byPublishedPort) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byPublishedPort) Less(i, j int) bool { return a[i].PublishedPort < a[j].PublishedPort }

func convertEndpointSpec(endpointMode string, source []composetypes.ServicePortConfig) (*swarm.EndpointSpec, error) {
	portConfigs := []swarm.PortConfig{}
	for _, port := range source {
		portConfig := swarm.PortConfig{
			Protocol:      swarm.PortConfigProtocol(port.Protocol),
			TargetPort:    port.Target,
			PublishedPort: port.Published,
			PublishMode:   swarm.PortConfigPublishMode(port.Mode),
		}
		portConfigs = append(portConfigs, portConfig)
	}

	sort.Sort(byPublishedPort(portConfigs))
	return &swarm.EndpointSpec{
		Mode:  swarm.ResolutionMode(strings.ToLower(endpointMode)),
		Ports: portConfigs,
	}, nil
}

func convertEnvironment(source map[string]*string) []string {
	var output []string

	for name, value := range source {
		switch value {
		case nil:
			output = append(output, name)
		default:
			output = append(output, fmt.Sprintf("%s=%s", name, *value))
		}
	}

	return output
}

func convertDeployMode(mode string, replicas *uint64) (swarm.ServiceMode, error) {
	serviceMode := swarm.ServiceMode{}

	switch mode {
	case "global":
		if replicas != nil {
			return serviceMode, errors.Errorf("replicas can only be used with replicated mode")
		}
		serviceMode.Global = &swarm.GlobalService{}
	case "replicated", "":
		serviceMode.Replicated = &swarm.ReplicatedService{Replicas: replicas}
	default:
		return serviceMode, errors.Errorf("Unknown mode: %s", mode)
	}
	return serviceMode, nil
}

func convertDNSConfig(DNS []string, DNSSearch []string) (*swarm.DNSConfig, error) {
	if DNS != nil || DNSSearch != nil {
		return &swarm.DNSConfig{
			Nameservers: DNS,
			Search:      DNSSearch,
		}, nil
	}
	return nil, nil
}
