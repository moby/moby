package types

import (
	"time"
)

// UnsupportedProperties not yet supported by this implementation of the compose file
var UnsupportedProperties = []string{
	"build",
	"cap_add",
	"cap_drop",
	"cgroup_parent",
	"devices",
	"dns",
	"dns_search",
	"domainname",
	"external_links",
	"ipc",
	"links",
	"mac_address",
	"network_mode",
	"privileged",
	"read_only",
	"restart",
	"security_opt",
	"shm_size",
	"stop_signal",
	"sysctls",
	"tmpfs",
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
	"extends":       "Support for `extends` is not implemented yet. Use `docker-compose config` to generate a configuration with all `extends` options resolved, and deploy from that.",
	"volume_driver": "Instead of setting the volume driver on the service, define a volume using the top-level `volumes` option and specify the driver there.",
	"volumes_from":  "To share a volume between services, define it using the top-level `volumes` option and reference it from each service that shares it using the service-level `volumes` option.",
	"cpu_quota":     "Set resource limits using deploy.resources",
	"cpu_shares":    "Set resource limits using deploy.resources",
	"cpuset":        "Set resource limits using deploy.resources",
	"mem_limit":     "Set resource limits using deploy.resources",
	"memswap_limit": "Set resource limits using deploy.resources",
}

// Dict is a mapping of strings to interface{}
type Dict map[string]interface{}

// ConfigFile is a filename and the contents of the file as a Dict
type ConfigFile struct {
	Filename string
	Config   Dict
}

// ConfigDetails are the details about a group of ConfigFiles
type ConfigDetails struct {
	WorkingDir  string
	ConfigFiles []ConfigFile
	Environment map[string]string
}

// Config is a full compose file configuration
type Config struct {
	Services []ServiceConfig
	Networks map[string]NetworkConfig
	Volumes  map[string]VolumeConfig
	Secrets  map[string]SecretConfig
}

// ServiceConfig is the configuration of one service
type ServiceConfig struct {
	Name string

	CapAdd          []string `mapstructure:"cap_add"`
	CapDrop         []string `mapstructure:"cap_drop"`
	CgroupParent    string   `mapstructure:"cgroup_parent"`
	Command         []string `compose:"shell_command"`
	ContainerName   string   `mapstructure:"container_name"`
	DependsOn       []string `mapstructure:"depends_on"`
	Deploy          DeployConfig
	Devices         []string
	DNS             []string          `compose:"string_or_list"`
	DNSSearch       []string          `mapstructure:"dns_search" compose:"string_or_list"`
	DomainName      string            `mapstructure:"domainname"`
	Entrypoint      []string          `compose:"shell_command"`
	Environment     map[string]string `compose:"list_or_dict_equals"`
	Expose          []string          `compose:"list_of_strings_or_numbers"`
	ExternalLinks   []string          `mapstructure:"external_links"`
	ExtraHosts      map[string]string `mapstructure:"extra_hosts" compose:"list_or_dict_colon"`
	Hostname        string
	HealthCheck     *HealthCheckConfig
	Image           string
	Ipc             string
	Labels          map[string]string `compose:"list_or_dict_equals"`
	Links           []string
	Logging         *LoggingConfig
	MacAddress      string                           `mapstructure:"mac_address"`
	NetworkMode     string                           `mapstructure:"network_mode"`
	Networks        map[string]*ServiceNetworkConfig `compose:"list_or_struct_map"`
	Pid             string
	Ports           []string `compose:"list_of_strings_or_numbers"`
	Privileged      bool
	ReadOnly        bool `mapstructure:"read_only"`
	Restart         string
	Secrets         []ServiceSecretConfig
	SecurityOpt     []string       `mapstructure:"security_opt"`
	StdinOpen       bool           `mapstructure:"stdin_open"`
	StopGracePeriod *time.Duration `mapstructure:"stop_grace_period"`
	StopSignal      string         `mapstructure:"stop_signal"`
	Tmpfs           []string       `compose:"string_or_list"`
	Tty             bool           `mapstructure:"tty"`
	Ulimits         map[string]*UlimitsConfig
	User            string
	Volumes         []string
	WorkingDir      string `mapstructure:"working_dir"`
}

// LoggingConfig the logging configuration for a service
type LoggingConfig struct {
	Driver  string
	Options map[string]string
}

// DeployConfig the deployment configuration for a service
type DeployConfig struct {
	Mode          string
	Replicas      *uint64
	Labels        map[string]string `compose:"list_or_dict_equals"`
	UpdateConfig  *UpdateConfig     `mapstructure:"update_config"`
	Resources     Resources
	RestartPolicy *RestartPolicy `mapstructure:"restart_policy"`
	Placement     Placement
}

// HealthCheckConfig the healthcheck configuration for a service
type HealthCheckConfig struct {
	Test     []string `compose:"healthcheck"`
	Timeout  string
	Interval string
	Retries  *uint64
	Disable  bool
}

// UpdateConfig the service update configuration
type UpdateConfig struct {
	Parallelism     *uint64
	Delay           time.Duration
	FailureAction   string `mapstructure:"failure_action"`
	Monitor         time.Duration
	MaxFailureRatio float32 `mapstructure:"max_failure_ratio"`
}

// Resources the resource limits and reservations
type Resources struct {
	Limits       *Resource
	Reservations *Resource
}

// Resource is a resource to be limited or reserved
type Resource struct {
	// TODO: types to convert from units and ratios
	NanoCPUs    string    `mapstructure:"cpus"`
	MemoryBytes UnitBytes `mapstructure:"memory"`
}

// UnitBytes is the bytes type
type UnitBytes int64

// RestartPolicy the service restart policy
type RestartPolicy struct {
	Condition   string
	Delay       *time.Duration
	MaxAttempts *uint64 `mapstructure:"max_attempts"`
	Window      *time.Duration
}

// Placement constraints for the service
type Placement struct {
	Constraints []string
}

// ServiceNetworkConfig is the network configuration for a service
type ServiceNetworkConfig struct {
	Aliases     []string
	Ipv4Address string `mapstructure:"ipv4_address"`
	Ipv6Address string `mapstructure:"ipv6_address"`
}

// ServiceSecretConfig is the secret configuration for a service
type ServiceSecretConfig struct {
	Source string
	Target string
	UID    string
	GID    string
	Mode   uint32
}

// UlimitsConfig the ulimit configuration
type UlimitsConfig struct {
	Single int
	Soft   int
	Hard   int
}

// NetworkConfig for a network
type NetworkConfig struct {
	Driver     string
	DriverOpts map[string]string `mapstructure:"driver_opts"`
	Ipam       IPAMConfig
	External   External
	Internal   bool
	Labels     map[string]string `compose:"list_or_dict_equals"`
}

// IPAMConfig for a network
type IPAMConfig struct {
	Driver string
	Config []*IPAMPool
}

// IPAMPool for a network
type IPAMPool struct {
	Subnet string
}

// VolumeConfig for a volume
type VolumeConfig struct {
	Driver     string
	DriverOpts map[string]string `mapstructure:"driver_opts"`
	External   External
	Labels     map[string]string `compose:"list_or_dict_equals"`
}

// External identifies a Volume or Network as a reference to a resource that is
// not managed, and should already exist.
type External struct {
	Name     string
	External bool
}

// SecretConfig for a secret
type SecretConfig struct {
	File     string
	External External
	Labels   map[string]string `compose:"list_or_dict_equals"`
}
