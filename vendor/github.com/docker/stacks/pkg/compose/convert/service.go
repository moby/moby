package convert

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/pkg/errors"

	"github.com/docker/stacks/pkg/compose/parser"
	composetypes "github.com/docker/stacks/pkg/compose/types"
	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/opts"
	"github.com/docker/stacks/pkg/types"
)

const (
	defaultNetwork = "default"
	// LabelImage is the label used to store image name provided in the compose file
	LabelImage = "com.docker.stack.image"
)

// Services converts all services defined in a StackSpec into a list of
// Swarm services.
func Services(
	namespace Namespace,
	stackSpec types.StackSpec,
	backend interfaces.SwarmResourceBackend,
) ([]swarm.ServiceSpec, error) {
	result := []swarm.ServiceSpec{}

	services := stackSpec.Services
	volumes := stackSpec.Volumes
	networks := stackSpec.Networks

	for _, service := range services {
		secrets, err := convertServiceSecrets(backend, namespace, service.Secrets, stackSpec.Secrets)
		if err != nil {
			return nil, errors.Wrapf(err, "service %s", service.Name)
		}

		configs, err := convertServiceConfigObjs(backend, namespace, service.Configs, stackSpec.Configs)
		if err != nil {
			return nil, errors.Wrapf(err, "service %s", service.Name)
		}

		serviceSpec, err := Service(namespace, service, networks, volumes, secrets, configs)
		if err != nil {
			return nil, errors.Wrapf(err, "service %s", service.Name)
		}

		serviceSpec.Annotations.Name = service.Name
		result = append(result, serviceSpec)
	}

	return result, nil
}

// Service converts a ServiceConfig into a swarm ServiceSpec
func Service(
	namespace Namespace,
	service composetypes.ServiceConfig,
	networkConfigs map[string]composetypes.NetworkConfig,
	volumes map[string]composetypes.VolumeConfig,
	secrets []*swarm.SecretReference,
	configs []*swarm.ConfigReference,
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

	var privileges swarm.Privileges
	privileges.CredentialSpec, err = convertCredentialSpec(service.CredentialSpec)
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
			ContainerSpec: &swarm.ContainerSpec{
				Image:           service.Image,
				Command:         service.Entrypoint,
				Args:            service.Command,
				Hostname:        service.Hostname,
				Hosts:           convertExtraHosts(service.ExtraHosts),
				DNSConfig:       dnsConfig,
				Healthcheck:     healthcheck,
				Env:             sortStrings(convertEnvironment(service.Environment)),
				Labels:          AddStackLabel(namespace, service.Labels),
				Dir:             service.WorkingDir,
				User:            service.User,
				Mounts:          mounts,
				StopGracePeriod: composetypes.ConvertDurationPtr(service.StopGracePeriod),
				StopSignal:      service.StopSignal,
				TTY:             service.Tty,
				OpenStdin:       service.StdinOpen,
				Secrets:         secrets,
				Configs:         configs,
				ReadOnly:        service.ReadOnly,
				Privileges:      &privileges,
				Isolation:       container.Isolation(service.Isolation),
				Init:            service.Init,
			},
			LogDriver:     logDriver,
			Resources:     resources,
			RestartPolicy: restartPolicy,
			Placement: &swarm.Placement{
				Constraints: service.Deploy.Placement.Constraints,
				Preferences: getPlacementPreference(service.Deploy.Placement.Preferences),
			},
		},
		EndpointSpec:   endpoint,
		Mode:           mode,
		UpdateConfig:   convertUpdateConfig(service.Deploy.UpdateConfig),
		RollbackConfig: convertUpdateConfig(service.Deploy.RollbackConfig),
	}

	// add an image label to serviceSpec
	serviceSpec.Labels[LabelImage] = service.Image
	serviceSpec.TaskTemplate.Networks = networks

	return serviceSpec, nil
}

func getPlacementPreference(preferences []composetypes.PlacementPreferences) []swarm.PlacementPreference {
	result := []swarm.PlacementPreference{}
	for _, preference := range preferences {
		spreadDescriptor := preference.Spread
		result = append(result, swarm.PlacementPreference{
			Spread: &swarm.SpreadOver{
				SpreadDescriptor: spreadDescriptor,
			},
		})
	}
	return result
}

func sortStrings(strs []string) []string {
	sort.Strings(strs)
	return strs
}

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
		if networkConfig.Name != "" {
			target = networkConfig.Name
		}
		netAttachConfig := swarm.NetworkAttachmentConfig{
			Target:  target,
			Aliases: aliases,
		}
		// Only add default aliases to user defined networks. Other networks do
		// not support aliases.
		if container.NetworkMode(target).IsUserDefined() {
			netAttachConfig.Aliases = append(netAttachConfig.Aliases, name)
		}
		nets = append(nets, netAttachConfig)
	}

	sort.Slice(nets, func(i, j int) bool {
		return nets[i].Target < nets[j].Target
	})
	return nets, nil
}

func convertServiceSecrets(
	backend interfaces.SwarmResourceBackend,
	namespace Namespace,
	secrets []composetypes.ServiceSecretConfig,
	secretSpecs map[string]composetypes.SecretConfig,
) ([]*swarm.SecretReference, error) {
	refs := []*swarm.SecretReference{}

	lookup := func(key string) (composetypes.FileObjectConfig, error) {
		secretSpec, exists := secretSpecs[key]
		if !exists {
			return composetypes.FileObjectConfig{}, errors.Errorf("undefined secret %q", key)
		}
		return composetypes.FileObjectConfig(secretSpec), nil
	}
	for _, secret := range secrets {
		obj, err := convertFileObject(namespace, composetypes.FileReferenceConfig(secret), lookup)
		if err != nil {
			return nil, err
		}

		file := swarm.SecretReferenceFileTarget(obj.File)
		refs = append(refs, &swarm.SecretReference{
			File:       &file,
			SecretName: obj.Name,
		})
	}

	secrs, err := parser.ParseSecrets(backend, refs)
	if err != nil {
		return nil, err
	}
	// sort to ensure idempotence (don't restart services just because the entries are in different order)
	sort.SliceStable(secrs, func(i, j int) bool { return secrs[i].SecretName < secrs[j].SecretName })
	return secrs, err
}

func convertServiceConfigObjs(
	backend interfaces.SwarmResourceBackend,
	namespace Namespace,
	configs []composetypes.ServiceConfigObjConfig,
	configSpecs map[string]composetypes.ConfigObjConfig,
) ([]*swarm.ConfigReference, error) {
	refs := []*swarm.ConfigReference{}

	lookup := func(key string) (composetypes.FileObjectConfig, error) {
		configSpec, exists := configSpecs[key]
		if !exists {
			return composetypes.FileObjectConfig{}, errors.Errorf("undefined config %q", key)
		}
		return composetypes.FileObjectConfig(configSpec), nil
	}
	for _, config := range configs {
		obj, err := convertFileObject(namespace, composetypes.FileReferenceConfig(config), lookup)
		if err != nil {
			return nil, err
		}

		file := swarm.ConfigReferenceFileTarget(obj.File)
		refs = append(refs, &swarm.ConfigReference{
			File:       &file,
			ConfigName: obj.Name,
		})
	}

	confs, err := parser.ParseConfigs(backend, refs)
	if err != nil {
		return nil, err
	}
	// sort to ensure idempotence (don't restart services just because the entries are in different order)
	sort.SliceStable(confs, func(i, j int) bool { return confs[i].ConfigName < confs[j].ConfigName })
	return confs, err
}

type swarmReferenceTarget struct {
	Name string
	UID  string
	GID  string
	Mode os.FileMode
}

type swarmReferenceObject struct {
	File swarmReferenceTarget
	ID   string
	Name string
}

func convertFileObject(
	namespace Namespace,
	config composetypes.FileReferenceConfig,
	lookup func(key string) (composetypes.FileObjectConfig, error),
) (swarmReferenceObject, error) {
	target := config.Target
	if target == "" {
		target = config.Source
	}

	obj, err := lookup(config.Source)
	if err != nil {
		return swarmReferenceObject{}, err
	}

	source := namespace.Scope(config.Source)
	if obj.Name != "" {
		source = obj.Name
	}

	uid := config.UID
	gid := config.GID
	if uid == "" {
		uid = "0"
	}
	if gid == "" {
		gid = "0"
	}
	mode := config.Mode
	if mode == nil {
		mode = uint32Ptr(0444)
	}

	return swarmReferenceObject{
		File: swarmReferenceTarget{
			Name: target,
			UID:  uid,
			GID:  gid,
			Mode: os.FileMode(*mode),
		},
		Name: source,
	}, nil
}

func uint32Ptr(value uint32) *uint32 {
	return &value
}

// convertExtraHosts converts <host>:<ip> mappings to SwarmKit notation:
// "IP-address hostname(s)". The original order of mappings is preserved.
func convertExtraHosts(extraHosts composetypes.HostsList) []string {
	hosts := []string{}
	for _, hostIP := range extraHosts {
		if v := strings.SplitN(hostIP, ":", 2); len(v) == 2 {
			// Convert to SwarmKit notation: IP-address hostname(s)
			hosts = append(hosts, fmt.Sprintf("%s %s", v[1], v[0]))
		}
	}
	return hosts
}

func convertHealthcheck(healthcheck *composetypes.HealthCheckConfig) (*container.HealthConfig, error) {
	if healthcheck == nil {
		return nil, nil
	}
	var (
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
	if healthcheck.Timeout != nil {
		timeout = time.Duration(*healthcheck.Timeout)
	}
	if healthcheck.Interval != nil {
		interval = time.Duration(*healthcheck.Interval)
	}
	if healthcheck.StartPeriod != nil {
		startPeriod = time.Duration(*healthcheck.StartPeriod)
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
		policy, err := opts.ParseRestartPolicy(restart)
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
		Delay:       composetypes.ConvertDurationPtr(source.Delay),
		MaxAttempts: source.MaxAttempts,
		Window:      composetypes.ConvertDurationPtr(source.Window),
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
		Delay:           time.Duration(source.Delay),
		FailureAction:   source.FailureAction,
		Monitor:         time.Duration(source.Monitor),
		MaxFailureRatio: source.MaxFailureRatio,
		Order:           source.Order,
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

		var generic []swarm.GenericResource
		for _, res := range source.Reservations.GenericResources {
			var r swarm.GenericResource

			if res.DiscreteResourceSpec != nil {
				r.DiscreteResourceSpec = &swarm.DiscreteGenericResource{
					Kind:  res.DiscreteResourceSpec.Kind,
					Value: res.DiscreteResourceSpec.Value,
				}
			}

			generic = append(generic, r)
		}

		resources.Reservations = &swarm.Resources{
			NanoCPUs:         cpus,
			MemoryBytes:      int64(source.Reservations.MemoryBytes),
			GenericResources: generic,
		}
	}
	return resources, nil
}

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

	sort.Slice(portConfigs, func(i, j int) bool {
		return portConfigs[i].PublishedPort < portConfigs[j].PublishedPort
	})

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

func convertCredentialSpec(spec composetypes.CredentialSpecConfig) (*swarm.CredentialSpec, error) {
	if spec.File == "" && spec.Registry == "" {
		return nil, nil
	}
	if spec.File != "" && spec.Registry != "" {
		return nil, errors.New("Invalid credential spec - must provide one of `File` or `Registry`")
	}
	swarmCredSpec := swarm.CredentialSpec(spec)
	return &swarmCredSpec, nil
}
