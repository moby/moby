package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// UnsupportedProperties not yet supported by this implementation of the compose file
var UnsupportedProperties = []string{
	"build",
	"cap_add",
	"cap_drop",
	"cgroup_parent",
	"devices",
	"domainname",
	"external_links",
	"ipc",
	"links",
	"mac_address",
	"network_mode",
	"pid",
	"privileged",
	"restart",
	"security_opt",
	"shm_size",
	"sysctls",
	"ulimits",
	"userns_mode",
}

// DeprecatedProperties that were removed from the v3 format, but their
// use should not impact the behaviour of the application.
var DeprecatedProperties = map[string]string{
	"container_name": "Setting the container name is not supported.",
	"expose":         "Exposing ports is unnecessary - services on the same network can access each other's containers on any port.",
}

// ForbiddenProperties that are not supported in this implementation of the
// compose file.
var ForbiddenProperties = map[string]string{
	"extends":       "Support for `extends` is not implemented yet.",
	"volume_driver": "Instead of setting the volume driver on the service, define a volume using the top-level `volumes` option and specify the driver there.",
	"volumes_from":  "To share a volume between services, define it using the top-level `volumes` option and reference it from each service that shares it using the service-level `volumes` option.",
	"cpu_quota":     "Set resource limits using deploy.resources",
	"cpu_shares":    "Set resource limits using deploy.resources",
	"cpuset":        "Set resource limits using deploy.resources",
	"mem_limit":     "Set resource limits using deploy.resources",
	"memswap_limit": "Set resource limits using deploy.resources",
}

// ConfigFile is a filename and the contents of the file as a Dict
type ConfigFile struct {
	Filename string
	Config   map[string]interface{}
}

// ConfigDetails are the details about a group of ConfigFiles
type ConfigDetails struct {
	Version     string
	WorkingDir  string
	ConfigFiles []ConfigFile
	Environment map[string]string
}

// Duration is a thin wrapper around time.Duration with improved JSON marshalling
type Duration time.Duration

func (d Duration) String() string {
	return time.Duration(d).String()
}

// ConvertDurationPtr converts a typedefined Duration pointer to a time.Duration pointer with the same value.
func ConvertDurationPtr(d *Duration) *time.Duration {
	if d == nil {
		return nil
	}
	res := time.Duration(*d)
	return &res
}

// MarshalJSON makes Duration implement json.Marshaler
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// MarshalYAML makes Duration implement yaml.Marshaler
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.String(), nil
}

// LookupEnv provides a lookup function for environment variables
func (cd ConfigDetails) LookupEnv(key string) (string, bool) {
	v, ok := cd.Environment[key]
	return v, ok
}

// Config is a full compose file configuration
type Config struct {
	Filename string                     `yaml:"-" json:"-"`
	Version  string                     `json:"version"`
	Services Services                   `json:"services"`
	Networks map[string]NetworkConfig   `yaml:",omitempty" json:"networks,omitempty"`
	Volumes  map[string]VolumeConfig    `yaml:",omitempty" json:"volumes,omitempty"`
	Secrets  map[string]SecretConfig    `yaml:",omitempty" json:"secrets,omitempty"`
	Configs  map[string]ConfigObjConfig `yaml:",omitempty" json:"configs,omitempty"`
	Extras   map[string]interface{}     `yaml:",inline" json:"-"`
}

// MarshalJSON makes Config implement json.Marshaler
func (c Config) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"version":  c.Version,
		"services": c.Services,
	}

	if len(c.Networks) > 0 {
		m["networks"] = c.Networks
	}
	if len(c.Volumes) > 0 {
		m["volumes"] = c.Volumes
	}
	if len(c.Secrets) > 0 {
		m["secrets"] = c.Secrets
	}
	if len(c.Configs) > 0 {
		m["configs"] = c.Configs
	}
	for k, v := range c.Extras {
		m[k] = v
	}
	return json.Marshal(m)
}

// Services is a list of ServiceConfig
type Services []ServiceConfig

// MarshalYAML makes Services implement yaml.Marshaller
func (s Services) MarshalYAML() (interface{}, error) {
	services := map[string]ServiceConfig{}
	for _, service := range s {
		services[service.Name] = service
	}
	return services, nil
}

// MarshalJSON makes Services implement json.Marshaler
func (s Services) MarshalJSON() ([]byte, error) {
	data, err := s.MarshalYAML()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(data, "", "  ")
}

// UnmarshalJSON makes Services implement json.Unmarshaler
func (s *Services) UnmarshalJSON(data []byte) error {
	services := map[string]ServiceConfig{}

	err := json.Unmarshal(data, &services)
	if err != nil {
		return err
	}
	for name, service := range services {
		service.Name = name
		*s = append(*s, service)
	}
	return nil
}

// ServiceConfig is the configuration of one service
type ServiceConfig struct {
	Name string `yaml:"-" json:"-"`

	Build           BuildConfig                      `yaml:",omitempty" json:"build,omitempty"`
	CapAdd          []string                         `mapstructure:"cap_add" yaml:"cap_add,omitempty" json:"cap_add,omitempty"`
	CapDrop         []string                         `mapstructure:"cap_drop" yaml:"cap_drop,omitempty" json:"cap_drop,omitempty"`
	CgroupParent    string                           `mapstructure:"cgroup_parent" yaml:"cgroup_parent,omitempty" json:"cgroup_parent,omitempty"`
	Command         ShellCommand                     `yaml:",omitempty" json:"command,omitempty"`
	Configs         []ServiceConfigObjConfig         `yaml:",omitempty" json:"configs,omitempty"`
	ContainerName   string                           `mapstructure:"container_name" yaml:"container_name,omitempty" json:"container_name,omitempty"`
	CredentialSpec  CredentialSpecConfig             `mapstructure:"credential_spec" yaml:"credential_spec,omitempty" json:"credential_spec,omitempty"`
	DependsOn       []string                         `mapstructure:"depends_on" yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Deploy          DeployConfig                     `yaml:",omitempty" json:"deploy,omitempty"`
	Devices         []string                         `yaml:",omitempty" json:"devices,omitempty"`
	DNS             StringList                       `yaml:",omitempty" json:"dns,omitempty"`
	DNSSearch       StringList                       `mapstructure:"dns_search" yaml:"dns_search,omitempty" json:"dns_search,omitempty"`
	DomainName      string                           `mapstructure:"domainname" yaml:"domainname,omitempty" json:"domainname,omitempty"`
	Entrypoint      ShellCommand                     `yaml:",omitempty" json:"entrypoint,omitempty"`
	Environment     MappingWithEquals                `yaml:",omitempty" json:"environment,omitempty"`
	EnvFile         StringList                       `mapstructure:"env_file" yaml:"env_file,omitempty" json:"env_file,omitempty"`
	Expose          StringOrNumberList               `yaml:",omitempty" json:"expose,omitempty"`
	ExternalLinks   []string                         `mapstructure:"external_links" yaml:"external_links,omitempty" json:"external_links,omitempty"`
	ExtraHosts      HostsList                        `mapstructure:"extra_hosts" yaml:"extra_hosts,omitempty" json:"extra_hosts,omitempty"`
	Hostname        string                           `yaml:",omitempty" json:"hostname,omitempty"`
	HealthCheck     *HealthCheckConfig               `yaml:",omitempty" json:"healthcheck,omitempty"`
	Image           string                           `yaml:",omitempty" json:"image,omitempty"`
	Init            *bool                            `yaml:",omitempty" json:"init,omitempty"`
	Ipc             string                           `yaml:",omitempty" json:"ipc,omitempty"`
	Isolation       string                           `mapstructure:"isolation" yaml:"isolation,omitempty" json:"isolation,omitempty"`
	Labels          Labels                           `yaml:",omitempty" json:"labels,omitempty"`
	Links           []string                         `yaml:",omitempty" json:"links,omitempty"`
	Logging         *LoggingConfig                   `yaml:",omitempty" json:"logging,omitempty"`
	MacAddress      string                           `mapstructure:"mac_address" yaml:"mac_address,omitempty" json:"mac_address,omitempty"`
	NetworkMode     string                           `mapstructure:"network_mode" yaml:"network_mode,omitempty" json:"network_mode,omitempty"`
	Networks        map[string]*ServiceNetworkConfig `yaml:",omitempty" json:"networks,omitempty"`
	Pid             string                           `yaml:",omitempty" json:"pid,omitempty"`
	Ports           []ServicePortConfig              `yaml:",omitempty" json:"ports,omitempty"`
	Privileged      bool                             `yaml:",omitempty" json:"privileged,omitempty"`
	ReadOnly        bool                             `mapstructure:"read_only" yaml:"read_only,omitempty" json:"read_only,omitempty"`
	Restart         string                           `yaml:",omitempty" json:"restart,omitempty"`
	Secrets         []ServiceSecretConfig            `yaml:",omitempty" json:"secrets,omitempty"`
	SecurityOpt     []string                         `mapstructure:"security_opt" yaml:"security_opt,omitempty" json:"security_opt,omitempty"`
	ShmSize         string                           `mapstructure:"shm_size" yaml:"shm_size,omitempty" json:"shm_size,omitempty"`
	StdinOpen       bool                             `mapstructure:"stdin_open" yaml:"stdin_open,omitempty" json:"stdin_open,omitempty"`
	StopGracePeriod *Duration                        `mapstructure:"stop_grace_period" yaml:"stop_grace_period,omitempty" json:"stop_grace_period,omitempty"`
	StopSignal      string                           `mapstructure:"stop_signal" yaml:"stop_signal,omitempty" json:"stop_signal,omitempty"`
	Sysctls         StringList                       `yaml:",omitempty" json:"sysctls,omitempty"`
	Tmpfs           StringList                       `yaml:",omitempty" json:"tmpfs,omitempty"`
	Tty             bool                             `mapstructure:"tty" yaml:"tty,omitempty" json:"tty,omitempty"`
	Ulimits         map[string]*UlimitsConfig        `yaml:",omitempty" json:"ulimits,omitempty"`
	User            string                           `yaml:",omitempty" json:"user,omitempty"`
	UserNSMode      string                           `mapstructure:"userns_mode" yaml:"userns_mode,omitempty" json:"userns_mode,omitempty"`
	Volumes         []ServiceVolumeConfig            `yaml:",omitempty" json:"volumes,omitempty"`
	WorkingDir      string                           `mapstructure:"working_dir" yaml:"working_dir,omitempty" json:"working_dir,omitempty"`

	Extras map[string]interface{} `yaml:",inline" json:"-"`
}

// BuildConfig is a type for build
// using the same format at libcompose: https://github.com/docker/libcompose/blob/master/yaml/build.go#L12
type BuildConfig struct {
	Context    string            `yaml:",omitempty" json:"context,omitempty"`
	Dockerfile string            `yaml:",omitempty" json:"dockerfile,omitempty"`
	Args       MappingWithEquals `yaml:",omitempty" json:"args,omitempty"`
	Labels     Labels            `yaml:",omitempty" json:"labels,omitempty"`
	CacheFrom  StringList        `mapstructure:"cache_from" yaml:"cache_from,omitempty" json:"cache_from,omitempty"`
	Network    string            `yaml:",omitempty" json:"network,omitempty"`
	Target     string            `yaml:",omitempty" json:"target,omitempty"`
}

// ShellCommand is a string or list of string args
type ShellCommand []string

// StringList is a type for fields that can be a string or list of strings
type StringList []string

// StringOrNumberList is a type for fields that can be a list of strings or
// numbers
type StringOrNumberList []string

// MappingWithEquals is a mapping type that can be converted from a list of
// key[=value] strings.
// For the key with an empty value (`key=`), the mapped value is set to a pointer to `""`.
// For the key without value (`key`), the mapped value is set to nil.
type MappingWithEquals map[string]*string

// Labels is a mapping type for labels
type Labels map[string]string

// MappingWithColon is a mapping type that can be converted from a list of
// 'key: value' strings
type MappingWithColon map[string]string

// HostsList is a list of colon-separated host-ip mappings
type HostsList []string

// LoggingConfig the logging configuration for a service
type LoggingConfig struct {
	Driver  string            `yaml:",omitempty" json:"driver,omitempty"`
	Options map[string]string `yaml:",omitempty" json:"options,omitempty"`
}

// DeployConfig the deployment configuration for a service
type DeployConfig struct {
	Mode           string         `yaml:",omitempty" json:"mode,omitempty"`
	Replicas       *uint64        `yaml:",omitempty" json:"replicas,omitempty"`
	Labels         Labels         `yaml:",omitempty" json:"labels,omitempty"`
	UpdateConfig   *UpdateConfig  `mapstructure:"update_config" yaml:"update_config,omitempty" json:"update_config,omitempty"`
	RollbackConfig *UpdateConfig  `mapstructure:"rollback_config" yaml:"rollback_config,omitempty" json:"rollback_config,omitempty"`
	Resources      Resources      `yaml:",omitempty" json:"resources,omitempty"`
	RestartPolicy  *RestartPolicy `mapstructure:"restart_policy" yaml:"restart_policy,omitempty" json:"restart_policy,omitempty"`
	Placement      Placement      `yaml:",omitempty" json:"placement,omitempty"`
	EndpointMode   string         `mapstructure:"endpoint_mode" yaml:"endpoint_mode,omitempty" json:"endpoint_mode,omitempty"`
}

// HealthCheckConfig the healthcheck configuration for a service
type HealthCheckConfig struct {
	Test        HealthCheckTest `yaml:",omitempty" json:"test,omitempty"`
	Timeout     *Duration       `yaml:",omitempty" json:"timeout,omitempty"`
	Interval    *Duration       `yaml:",omitempty" json:"interval,omitempty"`
	Retries     *uint64         `yaml:",omitempty" json:"retries,omitempty"`
	StartPeriod *Duration       `mapstructure:"start_period" yaml:"start_period,omitempty" json:"start_period,omitempty"`
	Disable     bool            `yaml:",omitempty" json:"disable,omitempty"`
}

// HealthCheckTest is the command run to test the health of a service
type HealthCheckTest []string

// UpdateConfig the service update configuration
type UpdateConfig struct {
	Parallelism     *uint64  `yaml:",omitempty" json:"parallelism,omitempty"`
	Delay           Duration `yaml:",omitempty" json:"delay,omitempty"`
	FailureAction   string   `mapstructure:"failure_action" yaml:"failure_action,omitempty" json:"failure_action,omitempty"`
	Monitor         Duration `yaml:",omitempty" json:"monitor,omitempty"`
	MaxFailureRatio float32  `mapstructure:"max_failure_ratio" yaml:"max_failure_ratio,omitempty" json:"max_failure_ratio,omitempty"`
	Order           string   `yaml:",omitempty" json:"order,omitempty"`
}

// Resources the resource limits and reservations
type Resources struct {
	Limits       *Resource `yaml:",omitempty" json:"limits,omitempty"`
	Reservations *Resource `yaml:",omitempty" json:"reservations,omitempty"`
}

// Resource is a resource to be limited or reserved
type Resource struct {
	// TODO: types to convert from units and ratios
	NanoCPUs         string            `mapstructure:"cpus" yaml:"cpus,omitempty" json:"cpus,omitempty"`
	MemoryBytes      UnitBytes         `mapstructure:"memory" yaml:"memory,omitempty" json:"memory,omitempty"`
	GenericResources []GenericResource `mapstructure:"generic_resources" yaml:"generic_resources,omitempty" json:"generic_resources,omitempty"`
}

// GenericResource represents a "user defined" resource which can
// only be an integer (e.g: SSD=3) for a service
type GenericResource struct {
	DiscreteResourceSpec *DiscreteGenericResource `mapstructure:"discrete_resource_spec" yaml:"discrete_resource_spec,omitempty" json:"discrete_resource_spec,omitempty"`
}

// DiscreteGenericResource represents a "user defined" resource which is defined
// as an integer
// "Kind" is used to describe the Kind of a resource (e.g: "GPU", "FPGA", "SSD", ...)
// Value is used to count the resource (SSD=5, HDD=3, ...)
type DiscreteGenericResource struct {
	Kind  string `json:"kind"`
	Value int64  `json:"value"`
}

// UnitBytes is the bytes type
type UnitBytes int64

// MarshalYAML makes UnitBytes implement yaml.Marshaller
func (u UnitBytes) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%d", u), nil
}

// MarshalJSON makes UnitBytes implement json.Marshaler
func (u UnitBytes) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%d"`, u)), nil
}

// RestartPolicy the service restart policy
type RestartPolicy struct {
	Condition   string    `yaml:",omitempty" json:"condition,omitempty"`
	Delay       *Duration `yaml:",omitempty" json:"delay,omitempty"`
	MaxAttempts *uint64   `mapstructure:"max_attempts" yaml:"max_attempts,omitempty" json:"max_attempts,omitempty"`
	Window      *Duration `yaml:",omitempty" json:"window,omitempty"`
}

// Placement constraints for the service
type Placement struct {
	Constraints []string               `yaml:",omitempty" json:"constraints,omitempty"`
	Preferences []PlacementPreferences `yaml:",omitempty" json:"preferences,omitempty"`
}

// PlacementPreferences is the preferences for a service placement
type PlacementPreferences struct {
	Spread string `yaml:",omitempty" json:"spread,omitempty"`
}

// ServiceNetworkConfig is the network configuration for a service
type ServiceNetworkConfig struct {
	Aliases     []string `yaml:",omitempty" json:"aliases,omitempty"`
	Ipv4Address string   `mapstructure:"ipv4_address" yaml:"ipv4_address,omitempty" json:"ipv4_address,omitempty"`
	Ipv6Address string   `mapstructure:"ipv6_address" yaml:"ipv6_address,omitempty" json:"ipv6_address,omitempty"`
}

// ServicePortConfig is the port configuration for a service
type ServicePortConfig struct {
	Mode      string `yaml:",omitempty" json:"mode,omitempty"`
	Target    uint32 `yaml:",omitempty" json:"target,omitempty"`
	Published uint32 `yaml:",omitempty" json:"published,omitempty"`
	Protocol  string `yaml:",omitempty" json:"protocol,omitempty"`

	// Variable holds a port variable definition when the actual
	// port settings aren't known until property substitution time
	// If non-empty, other fields are omitted
	Variable string `yaml:"-" json:"variable,omitempty"`
}

// ServiceVolumeConfig are references to a volume used by a service
type ServiceVolumeConfig struct {
	Type        string               `yaml:",omitempty" json:"type,omitempty"`
	Source      string               `yaml:",omitempty" json:"source,omitempty"`
	Target      string               `yaml:",omitempty" json:"target,omitempty"`
	ReadOnly    bool                 `mapstructure:"read_only" yaml:"read_only,omitempty" json:"read_only,omitempty"`
	Consistency string               `yaml:",omitempty" json:"consistency,omitempty"`
	Bind        *ServiceVolumeBind   `yaml:",omitempty" json:"bind,omitempty"`
	Volume      *ServiceVolumeVolume `yaml:",omitempty" json:"volume,omitempty"`
	Tmpfs       *ServiceVolumeTmpfs  `yaml:",omitempty" json:"tmpfs,omitempty"`
}

// ServiceVolumeBind are options for a service volume of type bind
type ServiceVolumeBind struct {
	Propagation string `yaml:",omitempty" json:"propagation,omitempty"`
}

// ServiceVolumeVolume are options for a service volume of type volume
type ServiceVolumeVolume struct {
	NoCopy bool `mapstructure:"nocopy" yaml:"nocopy,omitempty" json:"nocopy,omitempty"`
}

// ServiceVolumeTmpfs are options for a service volume of type tmpfs
type ServiceVolumeTmpfs struct {
	Size int64 `yaml:",omitempty" json:"size,omitempty"`
}

// FileReferenceConfig for a reference to a swarm file object
type FileReferenceConfig struct {
	Source string  `yaml:",omitempty" json:"source,omitempty"`
	Target string  `yaml:",omitempty" json:"target,omitempty"`
	UID    string  `yaml:",omitempty" json:"uid,omitempty"`
	GID    string  `yaml:",omitempty" json:"gid,omitempty"`
	Mode   *uint32 `yaml:",omitempty" json:"mode,omitempty"`
}

// ServiceConfigObjConfig is the config obj configuration for a service
type ServiceConfigObjConfig FileReferenceConfig

// ServiceSecretConfig is the secret configuration for a service
type ServiceSecretConfig FileReferenceConfig

// UlimitsConfig the ulimit configuration
type UlimitsConfig struct {
	Single int `yaml:",omitempty" json:"single,omitempty"`
	Soft   int `yaml:",omitempty" json:"soft,omitempty"`
	Hard   int `yaml:",omitempty" json:"hard,omitempty"`
}

// MarshalYAML makes UlimitsConfig implement yaml.Marshaller
func (u *UlimitsConfig) MarshalYAML() (interface{}, error) {
	if u.Single != 0 {
		return u.Single, nil
	}
	return u, nil
}

// MarshalJSON makes UlimitsConfig implement json.Marshaller
func (u *UlimitsConfig) MarshalJSON() ([]byte, error) {
	if u.Single != 0 {
		return json.Marshal(u.Single)
	}
	// Pass as a value to avoid re-entering this method and use the default implementation
	return json.Marshal(*u)
}

// NetworkConfig for a network
type NetworkConfig struct {
	Name       string                 `yaml:",omitempty" json:"name,omitempty"`
	Driver     string                 `yaml:",omitempty" json:"driver,omitempty"`
	DriverOpts map[string]string      `mapstructure:"driver_opts" yaml:"driver_opts,omitempty" json:"driver_opts,omitempty"`
	Ipam       IPAMConfig             `yaml:",omitempty" json:"ipam,omitempty"`
	External   External               `yaml:",omitempty" json:"external,omitempty"`
	Internal   bool                   `yaml:",omitempty" json:"internal,omitempty"`
	Attachable bool                   `yaml:",omitempty" json:"attachable,omitempty"`
	Labels     Labels                 `yaml:",omitempty" json:"labels,omitempty"`
	Extras     map[string]interface{} `yaml:",inline" json:"-"`
}

// IPAMConfig for a network
type IPAMConfig struct {
	Driver string      `yaml:",omitempty" json:"driver,omitempty"`
	Config []*IPAMPool `yaml:",omitempty" json:"config,omitempty"`
}

// IPAMPool for a network
type IPAMPool struct {
	Subnet string `yaml:",omitempty" json:"subnet,omitempty"`
}

// VolumeConfig for a volume
type VolumeConfig struct {
	Name       string                 `yaml:",omitempty" json:"name,omitempty"`
	Driver     string                 `yaml:",omitempty" json:"driver,omitempty"`
	DriverOpts map[string]string      `mapstructure:"driver_opts" yaml:"driver_opts,omitempty" json:"driver_opts,omitempty"`
	External   External               `yaml:",omitempty" json:"external,omitempty"`
	Labels     Labels                 `yaml:",omitempty" json:"labels,omitempty"`
	Extras     map[string]interface{} `yaml:",inline" json:"-"`
}

// External identifies a Volume or Network as a reference to a resource that is
// not managed, and should already exist.
// External.name is deprecated and replaced by Volume.name
type External struct {
	Name     string `yaml:",omitempty" json:"name,omitempty"`
	External bool   `yaml:",omitempty" json:"external,omitempty"`
}

// MarshalYAML makes External implement yaml.Marshaller
func (e External) MarshalYAML() (interface{}, error) {
	if e.Name == "" {
		return e.External, nil
	}
	return External{Name: e.Name}, nil
}

// MarshalJSON makes External implement json.Marshaller
func (e External) MarshalJSON() ([]byte, error) {
	if e.Name == "" {
		return []byte(fmt.Sprintf("%v", e.External)), nil
	}
	return []byte(fmt.Sprintf(`{"name": %q}`, e.Name)), nil
}

// UnmarshalJSON makes External implement json.Unmarshaller
func (e *External) UnmarshalJSON(data []byte) error {
	if strings.ToLower(string(data)) == "false" {
		e.External = false
		return nil
	} else if strings.ToLower(string(data)) == "true" {
		e.External = true
		return nil
	}
	nested := map[string]string{}
	err := json.Unmarshal(data, &nested)
	if err != nil {
		return err
	}
	name, ok := nested["name"]
	if !ok {
		return fmt.Errorf("malformed external json type: %s", string(data))
	}
	e.Name = name
	return nil
}

// CredentialSpecConfig for credential spec on Windows
type CredentialSpecConfig struct {
	Config   string `yaml:",omitempty" json:"config,omitempty"`
	File     string `yaml:",omitempty" json:"file,omitempty"`
	Registry string `yaml:",omitempty" json:"registry,omitempty"`
}

// FileObjectConfig is a config type for a file used by a service
type FileObjectConfig struct {
	Name     string                 `yaml:",omitempty" json:"name,omitempty"`
	File     string                 `yaml:",omitempty" json:"file,omitempty"`
	External External               `yaml:",omitempty" json:"external,omitempty"`
	Labels   Labels                 `yaml:",omitempty" json:"labels,omitempty"`
	Extras   map[string]interface{} `yaml:",inline" json:"-"`
}

// SecretConfig for a secret
type SecretConfig FileObjectConfig

// ConfigObjConfig is the config for the swarm "Config" object
type ConfigObjConfig FileObjectConfig
