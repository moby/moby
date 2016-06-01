package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/discovery"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/registry"
	"github.com/imdario/mergo"
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
	LogConfig
	bridgeConfig // bridgeConfig holds bridge network specific configuration.
	registry.ServiceOptions

	reloadLock sync.Mutex
	valuesSet  map[string]interface{}
}

// InstallCommonFlags adds command-line options to the top-level flag parser for
// the current process.
// Subsequent calls to `flag.Parse` will populate config with values parsed
// from the command-line.
func (config *Config) InstallCommonFlags(cmd *flag.FlagSet, usageFn func(string) string) {
	var maxConcurrentDownloads, maxConcurrentUploads int

	config.ServiceOptions.InstallCliFlags(cmd, usageFn)

	cmd.Var(opts.NewNamedListOptsRef("storage-opts", &config.GraphOptions, nil), []string{"-storage-opt"}, usageFn("Set storage driver options"))
	cmd.Var(opts.NewNamedListOptsRef("authorization-plugins", &config.AuthorizationPlugins, nil), []string{"-authorization-plugin"}, usageFn("List authorization plugins in order from first evaluator to last"))
	cmd.Var(opts.NewNamedListOptsRef("exec-opts", &config.ExecOptions, nil), []string{"-exec-opt"}, usageFn("Set runtime execution options"))
	cmd.StringVar(&config.Pidfile, []string{"p", "-pidfile"}, defaultPidFile, usageFn("Path to use for daemon PID file"))
	cmd.StringVar(&config.Root, []string{"g", "-graph"}, defaultGraph, usageFn("Root of the Docker runtime"))
	cmd.BoolVar(&config.AutoRestart, []string{"#r", "#-restart"}, true, usageFn("--restart on the daemon has been deprecated in favor of --restart policies on docker run"))
	cmd.StringVar(&config.GraphDriver, []string{"s", "-storage-driver"}, "", usageFn("Storage driver to use"))
	cmd.IntVar(&config.Mtu, []string{"#mtu", "-mtu"}, 0, usageFn("Set the containers network MTU"))
	cmd.BoolVar(&config.RawLogs, []string{"-raw-logs"}, false, usageFn("Full timestamps without ANSI coloring"))
	// FIXME: why the inconsistency between "hosts" and "sockets"?
	cmd.Var(opts.NewListOptsRef(&config.DNS, opts.ValidateIPAddress), []string{"#dns", "-dns"}, usageFn("DNS server to use"))
	cmd.Var(opts.NewNamedListOptsRef("dns-opts", &config.DNSOptions, nil), []string{"-dns-opt"}, usageFn("DNS options to use"))
	cmd.Var(opts.NewListOptsRef(&config.DNSSearch, opts.ValidateDNSSearch), []string{"-dns-search"}, usageFn("DNS search domains to use"))
	cmd.Var(opts.NewNamedListOptsRef("labels", &config.Labels, opts.ValidateLabel), []string{"-label"}, usageFn("Set key=value labels to the daemon"))
	cmd.StringVar(&config.LogConfig.Type, []string{"-log-driver"}, "json-file", usageFn("Default driver for container logs"))
	cmd.Var(opts.NewNamedMapOpts("log-opts", config.LogConfig.Config, nil), []string{"-log-opt"}, usageFn("Set log driver options"))
	cmd.StringVar(&config.ClusterAdvertise, []string{"-cluster-advertise"}, "", usageFn("Address or interface name to advertise"))
	cmd.StringVar(&config.ClusterStore, []string{"-cluster-store"}, "", usageFn("Set the cluster store"))
	cmd.Var(opts.NewNamedMapOpts("cluster-store-opts", config.ClusterOpts, nil), []string{"-cluster-store-opt"}, usageFn("Set cluster store options"))
	cmd.StringVar(&config.CorsHeaders, []string{"-api-cors-header"}, "", usageFn("Set CORS headers in the remote API"))
	cmd.IntVar(&maxConcurrentDownloads, []string{"-max-concurrent-downloads"}, defaultMaxConcurrentDownloads, usageFn("Set the max concurrent downloads for each pull"))
	cmd.IntVar(&maxConcurrentUploads, []string{"-max-concurrent-uploads"}, defaultMaxConcurrentUploads, usageFn("Set the max concurrent uploads for each push"))

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
func ReloadConfiguration(configFile string, flags *flag.FlagSet, reload func(*Config)) error {
	logrus.Infof("Got signal to reload configuration, reloading from: %s", configFile)
	newConfig, err := getConflictFreeConfiguration(configFile, flags)
	if err != nil {
		return err
	}

	if err := validateConfiguration(newConfig); err != nil {
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
func MergeDaemonConfigurations(flagsConfig *Config, flags *flag.FlagSet, configFile string) (*Config, error) {
	fileConfig, err := getConflictFreeConfiguration(configFile, flags)
	if err != nil {
		return nil, err
	}

	if err := validateConfiguration(fileConfig); err != nil {
		return nil, fmt.Errorf("file configuration validation failed (%v)", err)
	}

	// merge flags configuration on top of the file configuration
	if err := mergo.Merge(fileConfig, flagsConfig); err != nil {
		return nil, err
	}

	return fileConfig, nil
}

// getConflictFreeConfiguration loads the configuration from a JSON file.
// It compares that configuration with the one provided by the flags,
// and returns an error if there are conflicts.
func getConflictFreeConfiguration(configFile string, flags *flag.FlagSet) (*Config, error) {
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
		// are not overriden by default truthy values from the flags that were not explicitly set.
		// See https://github.com/docker/docker/issues/20289 for an example.
		//
		// TODO: Rewrite configuration logic to avoid same issue with other nullable values, like numbers.
		namedOptions := make(map[string]interface{})
		for key, value := range configSet {
			f := flags.Lookup("-" + key)
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
			flags.VisitAll(func(f *flag.Flag) {
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
func findConfigurationConflicts(config map[string]interface{}, flags *flag.FlagSet) error {
	// 1. Search keys from the file that we don't recognize as flags.
	unknownKeys := make(map[string]interface{})
	for key, value := range config {
		flagName := "-" + key
		if flag := flags.Lookup(flagName); flag == nil {
			unknownKeys[key] = value
		}
	}

	// 2. Discard values that implement NamedOption.
	// Their configuration name differs from their flag name, like `labels` and `label`.
	if len(unknownKeys) > 0 {
		unknownNamedConflicts := func(f *flag.Flag) {
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
	duplicatedConflicts := func(f *flag.Flag) {
		// search option name in the json configuration payload if the value is a named option
		if namedOption, ok := f.Value.(opts.NamedOption); ok {
			if optsValue, ok := config[namedOption.Name()]; ok {
				conflicts = append(conflicts, printConflict(namedOption.Name(), f.Value.String(), optsValue))
			}
		} else {
			// search flag name in the json configuration payload without trailing dashes
			for _, name := range f.Names {
				name = strings.TrimLeft(name, "-")

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

// validateConfiguration validates some specific configs.
// such as config.DNS, config.Labels, config.DNSSearch,
// as well as config.MaxConcurrentDownloads, config.MaxConcurrentUploads.
func validateConfiguration(config *Config) error {
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
	return nil
}
