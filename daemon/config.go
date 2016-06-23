package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/discovery"
	"github.com/docker/docker/registry"
	"github.com/imdario/mergo"
	"github.com/spf13/pflag"
)

const (
	// defaultMaxConcurrentDownloads is the default value for
	// maximum number of downloads that
	// may take place at a time for each pull.
	defaultMaxConcurrentDownloads = 3
	// defaultMaxConcurrentUploads is the default value for
	// maximum number of uploads that
	// may take place at a time for each push.
	defaultMaxConcurrentUploads = 5
	// stockRuntimeName is the reserved name/alias used to represent the
	// OCI runtime being shipped with the docker daemon package.
	stockRuntimeName = "runc"
)

const (
	defaultNetworkMtu    = 1500
	disableNetworkBridge = "none"
)

// flatOptions contains configuration keys
// that MUST NOT be parsed as deep structures.
// Use this to differentiate these options
// with others like the ones in CommonTLSOptions.
var flatOptions = map[string]bool{
	"cluster-store-opts": true,
	"log-opts":           true,
	"runtimes":           true,
}

// LogConfig represents the default log configuration.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line use.
type LogConfig struct {
	Type   string            `json:"log-driver,omitempty"`
	Config map[string]string `json:"log-opts,omitempty"`
}

// commonBridgeConfig stores all the platform-common bridge driver specific
// configuration.
type commonBridgeConfig struct {
	Iface     string `json:"bridge,omitempty"`
	FixedCIDR string `json:"fixed-cidr,omitempty"`
}

// CommonTLSOptions defines TLS configuration for the daemon server.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line use.
type CommonTLSOptions struct {
	CAFile   string `json:"tlscacert,omitempty"`
	CertFile string `json:"tlscert,omitempty"`
	KeyFile  string `json:"tlskey,omitempty"`
}

// CommonConfig defines the configuration of a docker daemon which is
// common across platforms.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line use.
type CommonConfig struct {
	AuthorizationPlugins []string            `json:"authorization-plugins,omitempty"` // AuthorizationPlugins holds list of authorization plugins
	AutoRestart          bool                `json:"-"`
	Context              map[string][]string `json:"-"`
	DisableBridge        bool                `json:"-"`
	DNS                  []string            `json:"dns,omitempty"`
	DNSOptions           []string            `json:"dns-opts,omitempty"`
	DNSSearch            []string            `json:"dns-search,omitempty"`
	ExecOptions          []string            `json:"exec-opts,omitempty"`
	GraphDriver          string              `json:"storage-driver,omitempty"`
	GraphOptions         []string            `json:"storage-opts,omitempty"`
	Labels               []string            `json:"labels,omitempty"`
	Mtu                  int                 `json:"mtu,omitempty"`
	Pidfile              string              `json:"pidfile,omitempty"`
	RawLogs              bool                `json:"raw-logs,omitempty"`
	Root                 string              `json:"graph,omitempty"`
	SocketGroup          string              `json:"group,omitempty"`
	TrustKeyPath         string              `json:"-"`
	CorsHeaders          string              `json:"api-cors-header,omitempty"`
	EnableCors           bool                `json:"api-enable-cors,omitempty"`

	// LiveRestoreEnabled determines whether we should keep containers
	// alive upon daemon shutdown/start
	LiveRestoreEnabled bool `json:"live-restore,omitempty"`

	// ClusterStore is the storage backend used for the cluster information. It is used by both
	// multihost networking (to store networks and endpoints information) and by the node discovery
	// mechanism.
	ClusterStore string `json:"cluster-store,omitempty"`

	// ClusterOpts is used to pass options to the discovery package for tuning libkv settings, such
	// as TLS configuration settings.
	ClusterOpts map[string]string `json:"cluster-store-opts,omitempty"`

	// ClusterAdvertise is the network endpoint that the Engine advertises for the purpose of node
	// discovery. This should be a 'host:port' combination on which that daemon instance is
	// reachable by other hosts.
	ClusterAdvertise string `json:"cluster-advertise,omitempty"`

	// MaxConcurrentDownloads is the maximum number of downloads that
	// may take place at a time for each pull.
	MaxConcurrentDownloads *int `json:"max-concurrent-downloads,omitempty"`

	// MaxConcurrentUploads is the maximum number of uploads that
	// may take place at a time for each push.
	MaxConcurrentUploads *int `json:"max-concurrent-uploads,omitempty"`

	Debug     bool     `json:"debug,omitempty"`
	Hosts     []string `json:"hosts,omitempty"`
	LogLevel  string   `json:"log-level,omitempty"`
	TLS       bool     `json:"tls,omitempty"`
	TLSVerify bool     `json:"tlsverify,omitempty"`

	// Embedded structs that allow config
	// deserialization without the full struct.
	CommonTLSOptions

	// SwarmDefaultAdvertiseAddr is the default host/IP or network interface
	// to use if a wildcard address is specified in the ListenAddr value
	// given to the /swarm/init endpoint and no advertise address is
	// specified.
	SwarmDefaultAdvertiseAddr string `json:"swarm-default-advertise-addr"`

	LogConfig
	bridgeConfig // bridgeConfig holds bridge network specific configuration.
	registry.ServiceOptions

	reloadLock sync.Mutex
	valuesSet  map[string]interface{}
}

// InstallCommonFlags adds flags to the pflag.FlagSet to configure the daemon
func (config *Config) InstallCommonFlags(flags *pflag.FlagSet) {
	var maxConcurrentDownloads, maxConcurrentUploads int

	config.ServiceOptions.InstallCliFlags(flags)

	flags.Var(opts.NewNamedListOptsRef("storage-opts", &config.GraphOptions, nil), "storage-opt", "Storage driver options")
	flags.Var(opts.NewNamedListOptsRef("authorization-plugins", &config.AuthorizationPlugins, nil), "authorization-plugin", "Authorization plugins to load")
	flags.Var(opts.NewNamedListOptsRef("exec-opts", &config.ExecOptions, nil), "exec-opt", "Runtime execution options")
	flags.StringVarP(&config.Pidfile, "pidfile", "p", defaultPidFile, "Path to use for daemon PID file")
	flags.StringVarP(&config.Root, "graph", "g", defaultGraph, "Root of the Docker runtime")
	flags.BoolVarP(&config.AutoRestart, "restart", "r", true, "--restart on the daemon has been deprecated in favor of --restart policies on docker run")
	flags.MarkDeprecated("restart", "Please use a restart policy on docker run")
	flags.StringVarP(&config.GraphDriver, "storage-driver", "s", "", "Storage driver to use")
	flags.IntVar(&config.Mtu, "mtu", 0, "Set the containers network MTU")
	flags.BoolVar(&config.RawLogs, "raw-logs", false, "Full timestamps without ANSI coloring")
	// FIXME: why the inconsistency between "hosts" and "sockets"?
	flags.Var(opts.NewListOptsRef(&config.DNS, opts.ValidateIPAddress), "dns", "DNS server to use")
	flags.Var(opts.NewNamedListOptsRef("dns-opts", &config.DNSOptions, nil), "dns-opt", "DNS options to use")
	flags.Var(opts.NewListOptsRef(&config.DNSSearch, opts.ValidateDNSSearch), "dns-search", "DNS search domains to use")
	flags.Var(opts.NewNamedListOptsRef("labels", &config.Labels, opts.ValidateLabel), "label", "Set key=value labels to the daemon")
	flags.StringVar(&config.LogConfig.Type, "log-driver", "json-file", "Default driver for container logs")
	flags.Var(opts.NewNamedMapOpts("log-opts", config.LogConfig.Config, nil), "log-opt", "Default log driver options for containers")
	flags.StringVar(&config.ClusterAdvertise, "cluster-advertise", "", "Address or interface name to advertise")
	flags.StringVar(&config.ClusterStore, "cluster-store", "", "URL of the distributed storage backend")
	flags.Var(opts.NewNamedMapOpts("cluster-store-opts", config.ClusterOpts, nil), "cluster-store-opt", "Set cluster store options")
	flags.StringVar(&config.CorsHeaders, "api-cors-header", "", "Set CORS headers in the remote API")
	flags.IntVar(&maxConcurrentDownloads, "max-concurrent-downloads", defaultMaxConcurrentDownloads, "Set the max concurrent downloads for each pull")
	flags.IntVar(&maxConcurrentUploads, "max-concurrent-uploads", defaultMaxConcurrentUploads, "Set the max concurrent uploads for each push")

	flags.StringVar(&config.SwarmDefaultAdvertiseAddr, "swarm-default-advertise-addr", "", "Set default address or interface for swarm advertised address")

	config.MaxConcurrentDownloads = &maxConcurrentDownloads
	config.MaxConcurrentUploads = &maxConcurrentUploads
}

// IsValueSet returns true if a configuration value
// was explicitly set in the configuration file.
func (config *Config) IsValueSet(name string) bool {
	if config.valuesSet == nil {
		return false
	}
	_, ok := config.valuesSet[name]
	return ok
}

// NewConfig returns a new fully initialized Config struct
func NewConfig() *Config {
	config := Config{}
	config.LogConfig.Config = make(map[string]string)
	config.ClusterOpts = make(map[string]string)

	if runtime.GOOS != "linux" {
		config.V2Only = true
	}
	return &config
}

func parseClusterAdvertiseSettings(clusterStore, clusterAdvertise string) (string, error) {
	if clusterAdvertise == "" {
		return "", errDiscoveryDisabled
	}
	if clusterStore == "" {
		return "", fmt.Errorf("invalid cluster configuration. --cluster-advertise must be accompanied by --cluster-store configuration")
	}

	advertise, err := discovery.ParseAdvertise(clusterAdvertise)
	if err != nil {
		return "", fmt.Errorf("discovery advertise parsing failed (%v)", err)
	}
	return advertise, nil
}

// ReloadConfiguration reads the configuration in the host and reloads the daemon and server.
func ReloadConfiguration(configFile string, flags *pflag.FlagSet, reload func(*Config)) error {
	logrus.Infof("Got signal to reload configuration, reloading from: %s", configFile)
	newConfig, err := getConflictFreeConfiguration(configFile, flags)
	if err != nil {
		return err
	}

	if err := ValidateConfiguration(newConfig); err != nil {
		return fmt.Errorf("file configuration validation failed (%v)", err)
	}

	reload(newConfig)
	return nil
}

// boolValue is an interface that boolean value flags implement
// to tell the command line how to make -name equivalent to -name=true.
type boolValue interface {
	IsBoolFlag() bool
}

// MergeDaemonConfigurations reads a configuration file,
// loads the file configuration in an isolated structure,
// and merges the configuration provided from flags on top
// if there are no conflicts.
func MergeDaemonConfigurations(flagsConfig *Config, flags *pflag.FlagSet, configFile string) (*Config, error) {
	fileConfig, err := getConflictFreeConfiguration(configFile, flags)
	if err != nil {
		return nil, err
	}

	if err := ValidateConfiguration(fileConfig); err != nil {
		return nil, fmt.Errorf("file configuration validation failed (%v)", err)
	}

	// merge flags configuration on top of the file configuration
	if err := mergo.Merge(fileConfig, flagsConfig); err != nil {
		return nil, err
	}

	// We need to validate again once both fileConfig and flagsConfig
	// have been merged
	if err := ValidateConfiguration(fileConfig); err != nil {
		return nil, fmt.Errorf("file configuration validation failed (%v)", err)
	}

	return fileConfig, nil
}

// getConflictFreeConfiguration loads the configuration from a JSON file.
// It compares that configuration with the one provided by the flags,
// and returns an error if there are conflicts.
func getConflictFreeConfiguration(configFile string, flags *pflag.FlagSet) (*Config, error) {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var config Config
	var reader io.Reader
	if flags != nil {
		var jsonConfig map[string]interface{}
		reader = bytes.NewReader(b)
		if err := json.NewDecoder(reader).Decode(&jsonConfig); err != nil {
			return nil, err
		}

		configSet := configValuesSet(jsonConfig)

		if err := findConfigurationConflicts(configSet, flags); err != nil {
			return nil, err
		}

		// Override flag values to make sure the values set in the config file with nullable values, like `false`,
		// are not overridden by default truthy values from the flags that were not explicitly set.
		// See https://github.com/docker/docker/issues/20289 for an example.
		//
		// TODO: Rewrite configuration logic to avoid same issue with other nullable values, like numbers.
		namedOptions := make(map[string]interface{})
		for key, value := range configSet {
			f := flags.Lookup(key)
			if f == nil { // ignore named flags that don't match
				namedOptions[key] = value
				continue
			}

			if _, ok := f.Value.(boolValue); ok {
				f.Value.Set(fmt.Sprintf("%v", value))
			}
		}
		if len(namedOptions) > 0 {
			// set also default for mergeVal flags that are boolValue at the same time.
			flags.VisitAll(func(f *pflag.Flag) {
				if opt, named := f.Value.(opts.NamedOption); named {
					v, set := namedOptions[opt.Name()]
					_, boolean := f.Value.(boolValue)
					if set && boolean {
						f.Value.Set(fmt.Sprintf("%v", v))
					}
				}
			})
		}

		config.valuesSet = configSet
	}

	reader = bytes.NewReader(b)
	err = json.NewDecoder(reader).Decode(&config)
	return &config, err
}

// configValuesSet returns the configuration values explicitly set in the file.
func configValuesSet(config map[string]interface{}) map[string]interface{} {
	flatten := make(map[string]interface{})
	for k, v := range config {
		if m, isMap := v.(map[string]interface{}); isMap && !flatOptions[k] {
			for km, vm := range m {
				flatten[km] = vm
			}
			continue
		}

		flatten[k] = v
	}
	return flatten
}

// findConfigurationConflicts iterates over the provided flags searching for
// duplicated configurations and unknown keys. It returns an error with all the conflicts if
// it finds any.
func findConfigurationConflicts(config map[string]interface{}, flags *pflag.FlagSet) error {
	// 1. Search keys from the file that we don't recognize as flags.
	unknownKeys := make(map[string]interface{})
	for key, value := range config {
		if flag := flags.Lookup(key); flag == nil {
			unknownKeys[key] = value
		}
	}

	// 2. Discard values that implement NamedOption.
	// Their configuration name differs from their flag name, like `labels` and `label`.
	if len(unknownKeys) > 0 {
		unknownNamedConflicts := func(f *pflag.Flag) {
			if namedOption, ok := f.Value.(opts.NamedOption); ok {
				if _, valid := unknownKeys[namedOption.Name()]; valid {
					delete(unknownKeys, namedOption.Name())
				}
			}
		}
		flags.VisitAll(unknownNamedConflicts)
	}

	if len(unknownKeys) > 0 {
		var unknown []string
		for key := range unknownKeys {
			unknown = append(unknown, key)
		}
		return fmt.Errorf("the following directives don't match any configuration option: %s", strings.Join(unknown, ", "))
	}

	var conflicts []string
	printConflict := func(name string, flagValue, fileValue interface{}) string {
		return fmt.Sprintf("%s: (from flag: %v, from file: %v)", name, flagValue, fileValue)
	}

	// 3. Search keys that are present as a flag and as a file option.
	duplicatedConflicts := func(f *pflag.Flag) {
		// search option name in the json configuration payload if the value is a named option
		if namedOption, ok := f.Value.(opts.NamedOption); ok {
			if optsValue, ok := config[namedOption.Name()]; ok {
				conflicts = append(conflicts, printConflict(namedOption.Name(), f.Value.String(), optsValue))
			}
		} else {
			// search flag name in the json configuration payload
			for _, name := range []string{f.Name, f.Shorthand} {
				if value, ok := config[name]; ok {
					conflicts = append(conflicts, printConflict(name, f.Value.String(), value))
					break
				}
			}
		}
	}

	flags.Visit(duplicatedConflicts)

	if len(conflicts) > 0 {
		return fmt.Errorf("the following directives are specified both as a flag and in the configuration file: %s", strings.Join(conflicts, ", "))
	}
	return nil
}

// ValidateConfiguration validates some specific configs.
// such as config.DNS, config.Labels, config.DNSSearch,
// as well as config.MaxConcurrentDownloads, config.MaxConcurrentUploads.
func ValidateConfiguration(config *Config) error {
	// validate DNS
	for _, dns := range config.DNS {
		if _, err := opts.ValidateIPAddress(dns); err != nil {
			return err
		}
	}

	// validate DNSSearch
	for _, dnsSearch := range config.DNSSearch {
		if _, err := opts.ValidateDNSSearch(dnsSearch); err != nil {
			return err
		}
	}

	// validate Labels
	for _, label := range config.Labels {
		if _, err := opts.ValidateLabel(label); err != nil {
			return err
		}
	}

	// validate MaxConcurrentDownloads
	if config.IsValueSet("max-concurrent-downloads") && config.MaxConcurrentDownloads != nil && *config.MaxConcurrentDownloads < 0 {
		return fmt.Errorf("invalid max concurrent downloads: %d", *config.MaxConcurrentDownloads)
	}

	// validate MaxConcurrentUploads
	if config.IsValueSet("max-concurrent-uploads") && config.MaxConcurrentUploads != nil && *config.MaxConcurrentUploads < 0 {
		return fmt.Errorf("invalid max concurrent uploads: %d", *config.MaxConcurrentUploads)
	}

	// validate that "default" runtime is not reset
	if runtimes := config.GetAllRuntimes(); len(runtimes) > 0 {
		if _, ok := runtimes[stockRuntimeName]; ok {
			return fmt.Errorf("runtime name '%s' is reserved", stockRuntimeName)
		}
	}

	if defaultRuntime := config.GetDefaultRuntimeName(); defaultRuntime != "" && defaultRuntime != stockRuntimeName {
		runtimes := config.GetAllRuntimes()
		if _, ok := runtimes[defaultRuntime]; !ok {
			return fmt.Errorf("specified default runtime '%s' does not exist", defaultRuntime)
		}
	}

	return nil
}
