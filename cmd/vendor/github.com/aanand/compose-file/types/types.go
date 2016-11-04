package types

import (
	"time"
)

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
	"tmpfs",
}

var DeprecatedProperties = map[string]string{
	"container_name": "Setting the container name is not supported.",
	"expose":         "Exposing ports is unnecessary - services on the same network can access each other's containers on any port.",
}

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

type Dict map[string]interface{}

type ConfigFile struct {
	Filename string
	Config   Dict
}

type ConfigDetails struct {
	WorkingDir  string
	ConfigFiles []ConfigFile
	Environment map[string]string
}

type Config struct {
	Services []ServiceConfig
	Networks map[string]NetworkConfig
	Volumes  map[string]VolumeConfig
}

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
	Dns             []string          `compose:"string_or_list"`
	DnsSearch       []string          `mapstructure:"dns_search" compose:"string_or_list"`
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

type LoggingConfig struct {
	Driver  string
	Options map[string]string
}

type DeployConfig struct {
	Mode          string
	Replicas      *uint64
	Labels        map[string]string `compose:"list_or_dict_equals"`
	UpdateConfig  *UpdateConfig     `mapstructure:"update_config"`
	Resources     Resources
	RestartPolicy *RestartPolicy `mapstructure:"restart_policy"`
	Placement     Placement
}

type HealthCheckConfig struct {
	Test     []string `compose:"healthcheck"`
	Timeout  string
	Interval string
	Retries  *uint64
	Disable  bool
}

type UpdateConfig struct {
	Parallelism     *uint64
	Delay           time.Duration
	FailureAction   string `mapstructure:"failure_action"`
	Monitor         time.Duration
	MaxFailureRatio float32 `mapstructure:"max_failure_ratio"`
}

type Resources struct {
	Limits       *Resource
	Reservations *Resource
}

type Resource struct {
	// TODO: types to convert from units and ratios
	NanoCPUs    string    `mapstructure:"cpus"`
	MemoryBytes UnitBytes `mapstructure:"memory"`
}

type UnitBytes int64

type RestartPolicy struct {
	Condition   string
	Delay       *time.Duration
	MaxAttempts *uint64 `mapstructure:"max_attempts"`
	Window      *time.Duration
}

type Placement struct {
	Constraints []string
}

type ServiceNetworkConfig struct {
	Aliases     []string
	Ipv4Address string `mapstructure:"ipv4_address"`
	Ipv6Address string `mapstructure:"ipv6_address"`
}

type UlimitsConfig struct {
	Single int
	Soft   int
	Hard   int
}

type NetworkConfig struct {
	Driver     string
	DriverOpts map[string]string `mapstructure:"driver_opts"`
	Ipam       IPAMConfig
	External   External
	Labels     map[string]string `compose:"list_or_dict_equals"`
}

type IPAMConfig struct {
	Driver string
	Config []*IPAMPool
}

type IPAMPool struct {
	Subnet string
}

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
