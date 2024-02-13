package config // import "github.com/docker/docker/daemon/config"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"dario.cat/mergo"
	"github.com/containerd/log"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/registry"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	// DefaultMaxConcurrentDownloads is the default value for
	// maximum number of downloads that
	// may take place at a time.
	DefaultMaxConcurrentDownloads = 3
	// DefaultMaxConcurrentUploads is the default value for
	// maximum number of uploads that
	// may take place at a time.
	DefaultMaxConcurrentUploads = 5
	// DefaultDownloadAttempts is the default value for
	// maximum number of attempts that
	// may take place at a time for each pull when the connection is lost.
	DefaultDownloadAttempts = 5
	// DefaultShmSize is the default value for container's shm size (64 MiB)
	DefaultShmSize int64 = 64 * 1024 * 1024
	// DefaultNetworkMtu is the default value for network MTU
	DefaultNetworkMtu = 1500
	// DisableNetworkBridge is the default value of the option to disable network bridge
	DisableNetworkBridge = "none"
	// DefaultShutdownTimeout is the default shutdown timeout (in seconds) for
	// the daemon for containers to stop when it is shutting down.
	DefaultShutdownTimeout = 15
	// DefaultInitBinary is the name of the default init binary
	DefaultInitBinary = "docker-init"
	// DefaultRuntimeBinary is the default runtime to be used by
	// containerd if none is specified
	DefaultRuntimeBinary = "runc"
	// DefaultContainersNamespace is the name of the default containerd namespace used for users containers.
	DefaultContainersNamespace = "moby"
	// DefaultPluginNamespace is the name of the default containerd namespace used for plugins.
	DefaultPluginNamespace = "plugins.moby"
	// defaultMinAPIVersion is the minimum API version supported by the API.
	// This version can be overridden through the "DOCKER_MIN_API_VERSION"
	// environment variable. It currently defaults to the minimum API version
	// supported by the API server.
	defaultMinAPIVersion = api.MinSupportedAPIVersion
	// SeccompProfileDefault is the built-in default seccomp profile.
	SeccompProfileDefault = "builtin"
	// SeccompProfileUnconfined is a special profile name for seccomp to use an
	// "unconfined" seccomp profile.
	SeccompProfileUnconfined = "unconfined"
)

// flatOptions contains configuration keys
// that MUST NOT be parsed as deep structures.
// Use this to differentiate these options
// with others like the ones in TLSOptions.
var flatOptions = map[string]bool{
	"cluster-store-opts":   true,
	"default-network-opts": true,
	"log-opts":             true,
	"runtimes":             true,
	"default-ulimits":      true,
	"features":             true,
	"builder":              true,
}

// skipValidateOptions contains configuration keys
// that will be skipped from findConfigurationConflicts
// for unknown flag validation.
var skipValidateOptions = map[string]bool{
	"features": true,
	"builder":  true,
	// Corresponding flag has been removed because it was already unusable
	"deprecated-key-path": true,
}

// skipDuplicates contains configuration keys that
// will be skipped when checking duplicated
// configuration field defined in both daemon
// config file and from dockerd cli flags.
// This allows some configurations to be merged
// during the parsing.
var skipDuplicates = map[string]bool{
	"runtimes": true,
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

// NetworkConfig stores the daemon-wide networking configurations
type NetworkConfig struct {
	// Default address pools for docker networks
	DefaultAddressPools opts.PoolsOpt `json:"default-address-pools,omitempty"`
	// NetworkControlPlaneMTU allows to specify the control plane MTU, this will allow to optimize the network use in some components
	NetworkControlPlaneMTU int `json:"network-control-plane-mtu,omitempty"`
	// Default options for newly created networks
	DefaultNetworkOpts map[string]map[string]string `json:"default-network-opts,omitempty"`
}

// TLSOptions defines TLS configuration for the daemon server.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line use.
type TLSOptions struct {
	CAFile   string `json:"tlscacert,omitempty"`
	CertFile string `json:"tlscert,omitempty"`
	KeyFile  string `json:"tlskey,omitempty"`
}

// DNSConfig defines the DNS configurations.
type DNSConfig struct {
	DNS           []net.IP `json:"dns,omitempty"`
	DNSOptions    []string `json:"dns-opts,omitempty"`
	DNSSearch     []string `json:"dns-search,omitempty"`
	HostGatewayIP net.IP   `json:"host-gateway-ip,omitempty"`
}

// CommonConfig defines the configuration of a docker daemon which is
// common across platforms.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line use.
type CommonConfig struct {
	AuthorizationPlugins  []string `json:"authorization-plugins,omitempty"` // AuthorizationPlugins holds list of authorization plugins
	AutoRestart           bool     `json:"-"`
	DisableBridge         bool     `json:"-"`
	ExecOptions           []string `json:"exec-opts,omitempty"`
	GraphDriver           string   `json:"storage-driver,omitempty"`
	GraphOptions          []string `json:"storage-opts,omitempty"`
	Labels                []string `json:"labels,omitempty"`
	NetworkDiagnosticPort int      `json:"network-diagnostic-port,omitempty"`
	Pidfile               string   `json:"pidfile,omitempty"`
	RawLogs               bool     `json:"raw-logs,omitempty"`
	Root                  string   `json:"data-root,omitempty"`
	ExecRoot              string   `json:"exec-root,omitempty"`
	SocketGroup           string   `json:"group,omitempty"`
	CorsHeaders           string   `json:"api-cors-header,omitempty"`

	// Proxies holds the proxies that are configured for the daemon.
	Proxies `json:"proxies"`

	// LiveRestoreEnabled determines whether we should keep containers
	// alive upon daemon shutdown/start
	LiveRestoreEnabled bool `json:"live-restore,omitempty"`

	// MaxConcurrentDownloads is the maximum number of downloads that
	// may take place at a time for each pull.
	MaxConcurrentDownloads int `json:"max-concurrent-downloads,omitempty"`

	// MaxConcurrentUploads is the maximum number of uploads that
	// may take place at a time for each push.
	MaxConcurrentUploads int `json:"max-concurrent-uploads,omitempty"`

	// MaxDownloadAttempts is the maximum number of attempts that
	// may take place at a time for each push.
	MaxDownloadAttempts int `json:"max-download-attempts,omitempty"`

	// ShutdownTimeout is the timeout value (in seconds) the daemon will wait for the container
	// to stop when daemon is being shutdown
	ShutdownTimeout int `json:"shutdown-timeout,omitempty"`

	Debug     bool             `json:"debug,omitempty"`
	Hosts     []string         `json:"hosts,omitempty"`
	LogLevel  string           `json:"log-level,omitempty"`
	LogFormat log.OutputFormat `json:"log-format,omitempty"`
	TLS       *bool            `json:"tls,omitempty"`
	TLSVerify *bool            `json:"tlsverify,omitempty"`

	// Embedded structs that allow config
	// deserialization without the full struct.
	TLSOptions

	// SwarmDefaultAdvertiseAddr is the default host/IP or network interface
	// to use if a wildcard address is specified in the ListenAddr value
	// given to the /swarm/init endpoint and no advertise address is
	// specified.
	SwarmDefaultAdvertiseAddr string `json:"swarm-default-advertise-addr"`

	// SwarmRaftHeartbeatTick is the number of ticks in time for swarm mode raft quorum heartbeat
	// Typical value is 1
	SwarmRaftHeartbeatTick uint32 `json:"swarm-raft-heartbeat-tick"`

	// SwarmRaftElectionTick is the number of ticks to elapse before followers in the quorum can propose
	// a new round of leader election.  Default, recommended value is at least 10X that of Heartbeat tick.
	// Higher values can make the quorum less sensitive to transient faults in the environment, but this also
	// means it takes longer for the managers to detect a down leader.
	SwarmRaftElectionTick uint32 `json:"swarm-raft-election-tick"`

	MetricsAddress string `json:"metrics-addr"`

	DNSConfig
	LogConfig
	BridgeConfig // BridgeConfig holds bridge network specific configuration.
	NetworkConfig
	registry.ServiceOptions

	// FIXME(vdemeester) This part is not that clear and is mainly dependent on cli flags
	// It should probably be handled outside this package.
	ValuesSet map[string]interface{} `json:"-"`

	Experimental bool `json:"experimental"` // Experimental indicates whether experimental features should be exposed or not

	// Exposed node Generic Resources
	// e.g: ["orange=red", "orange=green", "orange=blue", "apple=3"]
	NodeGenericResources []string `json:"node-generic-resources,omitempty"`

	// ContainerAddr is the address used to connect to containerd if we're
	// not starting it ourselves
	ContainerdAddr string `json:"containerd,omitempty"`

	// CriContainerd determines whether a supervised containerd instance
	// should be configured with the CRI plugin enabled. This allows using
	// Docker's containerd instance directly with a Kubernetes kubelet.
	CriContainerd bool `json:"cri-containerd,omitempty"`

	// Features contains a list of feature key value pairs indicating what features are enabled or disabled.
	// If a certain feature doesn't appear in this list then it's unset (i.e. neither true nor false).
	Features map[string]bool `json:"features,omitempty"`

	Builder BuilderConfig `json:"builder,omitempty"`

	ContainerdNamespace       string `json:"containerd-namespace,omitempty"`
	ContainerdPluginNamespace string `json:"containerd-plugin-namespace,omitempty"`

	DefaultRuntime string `json:"default-runtime,omitempty"`

	// CDISpecDirs is a list of directories in which CDI specifications can be found.
	CDISpecDirs []string `json:"cdi-spec-dirs,omitempty"`

	// The minimum API version provided by the daemon. Defaults to [defaultMinAPIVersion].
	//
	// The DOCKER_MIN_API_VERSION allows overriding the minimum API version within
	// constraints of the minimum and maximum (current) supported API versions.
	//
	// API versions older than [defaultMinAPIVersion] are deprecated and
	// to be removed in a future release. The "DOCKER_MIN_API_VERSION" env
	// var should only be used for exceptional cases, and the MinAPIVersion
	// field is therefore not included in the JSON representation.
	MinAPIVersion string `json:"-"`
}

// Proxies holds the proxies that are configured for the daemon.
type Proxies struct {
	HTTPProxy  string `json:"http-proxy,omitempty"`
	HTTPSProxy string `json:"https-proxy,omitempty"`
	NoProxy    string `json:"no-proxy,omitempty"`
}

// IsValueSet returns true if a configuration value
// was explicitly set in the configuration file.
func (conf *Config) IsValueSet(name string) bool {
	if conf.ValuesSet == nil {
		return false
	}
	_, ok := conf.ValuesSet[name]
	return ok
}

// New returns a new fully initialized Config struct with default values set.
func New() (*Config, error) {
	// platform-agnostic default values for the Config.
	cfg := &Config{
		CommonConfig: CommonConfig{
			ShutdownTimeout: DefaultShutdownTimeout,
			LogConfig: LogConfig{
				Config: make(map[string]string),
			},
			MaxConcurrentDownloads: DefaultMaxConcurrentDownloads,
			MaxConcurrentUploads:   DefaultMaxConcurrentUploads,
			MaxDownloadAttempts:    DefaultDownloadAttempts,
			BridgeConfig: BridgeConfig{
				DefaultBridgeConfig: DefaultBridgeConfig{
					MTU: DefaultNetworkMtu,
				},
			},
			NetworkConfig: NetworkConfig{
				NetworkControlPlaneMTU: DefaultNetworkMtu,
				DefaultNetworkOpts:     make(map[string]map[string]string),
			},
			ContainerdNamespace:       DefaultContainersNamespace,
			ContainerdPluginNamespace: DefaultPluginNamespace,
			DefaultRuntime:            StockRuntimeName,
			MinAPIVersion:             defaultMinAPIVersion,
		},
	}

	if err := setPlatformDefaults(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// GetConflictFreeLabels validates Labels for conflict
// In swarm the duplicates for labels are removed
// so we only take same values here, no conflict values
// If the key-value is the same we will only take the last label
func GetConflictFreeLabels(labels []string) ([]string, error) {
	labelMap := map[string]string{}
	for _, label := range labels {
		key, val, ok := strings.Cut(label, "=")
		if ok {
			// If there is a conflict we will return an error
			if v, ok := labelMap[key]; ok && v != val {
				return nil, errors.Errorf("conflict labels for %s=%s and %s=%s", key, val, key, v)
			}
			labelMap[key] = val
		}
	}

	newLabels := []string{}
	for k, v := range labelMap {
		newLabels = append(newLabels, k+"="+v)
	}
	return newLabels, nil
}

// Reload reads the configuration in the host and reloads the daemon and server.
func Reload(configFile string, flags *pflag.FlagSet, reload func(*Config)) error {
	log.G(context.TODO()).Infof("Got signal to reload configuration, reloading from: %s", configFile)
	newConfig, err := getConflictFreeConfiguration(configFile, flags)
	if err != nil {
		if flags.Changed("config-file") || !os.IsNotExist(err) {
			return errors.Wrapf(err, "unable to configure the Docker daemon with file %s", configFile)
		}
		newConfig, err = New()
		if err != nil {
			return err
		}
	}

	// Check if duplicate label-keys with different values are found
	newLabels, err := GetConflictFreeLabels(newConfig.Labels)
	if err != nil {
		return err
	}
	newConfig.Labels = newLabels

	// TODO(thaJeztah) This logic is problematic and needs a rewrite;
	// This is validating newConfig before the "reload()" callback is executed.
	// At this point, newConfig may be a partial configuration, to be merged
	// with the existing configuration in the "reload()" callback. Validating
	// this config before it's merged can result in incorrect validation errors.
	//
	// However, the current "reload()" callback we use is DaemonCli.reloadConfig(),
	// which includes a call to Daemon.Reload(), which both performs "merging"
	// and validation, as well as actually updating the daemon configuration.
	// Calling DaemonCli.reloadConfig() *before* validation, could thus lead to
	// a failure in that function (making the reload non-atomic).
	//
	// While *some* errors could always occur when applying/updating the config,
	// we should make it more atomic, and;
	//
	// 1. get (a copy of) the active configuration
	// 2. get the new configuration
	// 3. apply the (reloadable) options from the new configuration
	// 4. validate the merged results
	// 5. apply the new configuration.
	if err := Validate(newConfig); err != nil {
		return errors.Wrap(err, "file configuration validation failed")
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

	// merge flags configuration on top of the file configuration
	if err := mergo.Merge(fileConfig, flagsConfig); err != nil {
		return nil, err
	}

	// validate the merged fileConfig and flagsConfig
	if err := Validate(fileConfig); err != nil {
		return nil, errors.Wrap(err, "merged configuration validation from file and command line flags failed")
	}

	return fileConfig, nil
}

// getConflictFreeConfiguration loads the configuration from a JSON file.
// It compares that configuration with the one provided by the flags,
// and returns an error if there are conflicts.
func getConflictFreeConfiguration(configFile string, flags *pflag.FlagSet) (*Config, error) {
	b, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	// Decode the contents of the JSON file using a [byte order mark] if present, instead of assuming UTF-8 without BOM.
	// The BOM, if present, will be used to determine the encoding. If no BOM is present, we will assume the default
	// and preferred encoding for JSON as defined by [RFC 8259], UTF-8 without BOM.
	//
	// While JSON is normatively UTF-8 with no BOM, there are a couple of reasons to decode here:
	//   * UTF-8 with BOM is something that new implementations should avoid producing; however, [RFC 8259 Section 8.1]
	//     allows implementations to ignore the UTF-8 BOM when present for interoperability. Older versions of Notepad,
	//     the only text editor available out of the box on Windows Server, writes UTF-8 with a BOM by default.
	//   * The default encoding for [Windows PowerShell] is UTF-16 LE with BOM. While encodings in PowerShell can be a
	//     bit idiosyncratic, BOMs are still generally written. There is no support for selecting UTF-8 without a BOM as
	//     the encoding in Windows PowerShell, though some Cmdlets only write UTF-8 with no BOM. PowerShell Core
	//     introduces `utf8NoBOM` and makes it the default, but PowerShell Core is unlikely to be the implementation for
	//     a majority of Windows Server + PowerShell users.
	//   * While [RFC 8259 Section 8.1] asserts that software that is not part of a closed ecosystem or that crosses a
	//     network boundary should only support UTF-8, and should never write a BOM, it does acknowledge older versions
	//     of the standard, such as [RFC 7159 Section 8.1]. In the interest of pragmatism and easing pain for Windows
	//     users, we consider Windows tools such as Windows PowerShell and Notepad part of our ecosystem, and support
	//     the two most common encodings: UTF-16 LE with BOM, and UTF-8 with BOM, in addition to the standard UTF-8
	//     without BOM.
	//
	// [byte order mark]: https://www.unicode.org/faq/utf_bom.html#BOM
	// [RFC 8259]: https://www.rfc-editor.org/rfc/rfc8259
	// [RFC 8259 Section 8.1]: https://www.rfc-editor.org/rfc/rfc8259#section-8.1
	// [RFC 7159 Section 8.1]: https://www.rfc-editor.org/rfc/rfc7159#section-8.1
	// [Windows PowerShell]: https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_character_encoding?view=powershell-5.1
	b, n, err := transform.Bytes(transform.Chain(unicode.BOMOverride(transform.Nop), encoding.UTF8Validator), b)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode configuration JSON at offset %d", n)
	}
	// Trim whitespace so that an empty config can be detected for an early return.
	b = bytes.TrimSpace(b)

	var config Config
	if len(b) == 0 {
		return &config, nil // early return on empty config
	}

	if flags != nil {
		var jsonConfig map[string]interface{}
		if err := json.Unmarshal(b, &jsonConfig); err != nil {
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

		config.ValuesSet = configSet
	}

	if err := json.Unmarshal(b, &config); err != nil {
		return nil, err
	}

	return &config, nil
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
		if flag := flags.Lookup(key); flag == nil && !skipValidateOptions[key] {
			unknownKeys[key] = value
		}
	}

	// 2. Discard values that implement NamedOption.
	// Their configuration name differs from their flag name, like `labels` and `label`.
	if len(unknownKeys) > 0 {
		unknownNamedConflicts := func(f *pflag.Flag) {
			if namedOption, ok := f.Value.(opts.NamedOption); ok {
				delete(unknownKeys, namedOption.Name())
			}
		}
		flags.VisitAll(unknownNamedConflicts)
	}

	if len(unknownKeys) > 0 {
		var unknown []string
		for key := range unknownKeys {
			unknown = append(unknown, key)
		}
		return errors.Errorf("the following directives don't match any configuration option: %s", strings.Join(unknown, ", "))
	}

	var conflicts []string
	printConflict := func(name string, flagValue, fileValue interface{}) string {
		switch name {
		case "http-proxy", "https-proxy":
			flagValue = MaskCredentials(flagValue.(string))
			fileValue = MaskCredentials(fileValue.(string))
		}
		return fmt.Sprintf("%s: (from flag: %v, from file: %v)", name, flagValue, fileValue)
	}

	// 3. Search keys that are present as a flag and as a file option.
	duplicatedConflicts := func(f *pflag.Flag) {
		// search option name in the json configuration payload if the value is a named option
		if namedOption, ok := f.Value.(opts.NamedOption); ok {
			if optsValue, ok := config[namedOption.Name()]; ok && !skipDuplicates[namedOption.Name()] {
				conflicts = append(conflicts, printConflict(namedOption.Name(), f.Value.String(), optsValue))
			}
		} else {
			// search flag name in the json configuration payload
			for _, name := range []string{f.Name, f.Shorthand} {
				if value, ok := config[name]; ok && !skipDuplicates[name] {
					conflicts = append(conflicts, printConflict(name, f.Value.String(), value))
					break
				}
			}
		}
	}

	flags.Visit(duplicatedConflicts)

	if len(conflicts) > 0 {
		return errors.Errorf("the following directives are specified both as a flag and in the configuration file: %s", strings.Join(conflicts, ", "))
	}
	return nil
}

// ValidateMinAPIVersion verifies if the given API version is within the
// range supported by the daemon. It is used to validate a custom minimum
// API version set through DOCKER_MIN_API_VERSION.
func ValidateMinAPIVersion(ver string) error {
	if ver == "" {
		return errors.New(`value is empty`)
	}
	if strings.EqualFold(ver[0:1], "v") {
		return errors.New(`API version must be provided without "v" prefix`)
	}
	if versions.LessThan(ver, defaultMinAPIVersion) {
		return errors.Errorf(`minimum supported API version is %s: %s`, defaultMinAPIVersion, ver)
	}
	if versions.GreaterThan(ver, api.DefaultVersion) {
		return errors.Errorf(`maximum supported API version is %s: %s`, api.DefaultVersion, ver)
	}
	return nil
}

// Validate validates some specific configs.
// such as config.DNS, config.Labels, config.DNSSearch,
// as well as config.MaxConcurrentDownloads, config.MaxConcurrentUploads and config.MaxDownloadAttempts.
func Validate(config *Config) error {
	// validate log-level
	if config.LogLevel != "" {
		// FIXME(thaJeztah): find a better way for this; this depends on knowledge of containerd's log package internals.
		// Alternatively: try  log.SetLevel(config.LogLevel), and restore the original level, but this also requires internal knowledge.
		switch strings.ToLower(config.LogLevel) {
		case "panic", "fatal", "error", "warn", "info", "debug", "trace":
			// These are valid. See [log.SetLevel] for a list of accepted levels.
		default:
			return errors.Errorf("invalid logging level: %s", config.LogLevel)
		}
	}

	// validate log-format
	if logFormat := config.LogFormat; logFormat != "" {
		switch logFormat {
		case log.TextFormat, log.JSONFormat:
			// These are valid
		default:
			return errors.Errorf("invalid log format: %s", logFormat)
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

	// TODO(thaJeztah) Validations below should not accept "0" to be valid; see Validate() for a more in-depth description of this problem
	if config.MTU < 0 {
		return errors.Errorf("invalid default MTU: %d", config.MTU)
	}
	if config.MaxConcurrentDownloads < 0 {
		return errors.Errorf("invalid max concurrent downloads: %d", config.MaxConcurrentDownloads)
	}
	if config.MaxConcurrentUploads < 0 {
		return errors.Errorf("invalid max concurrent uploads: %d", config.MaxConcurrentUploads)
	}
	if config.MaxDownloadAttempts < 0 {
		return errors.Errorf("invalid max download attempts: %d", config.MaxDownloadAttempts)
	}

	if _, err := ParseGenericResources(config.NodeGenericResources); err != nil {
		return err
	}

	for _, h := range config.Hosts {
		if _, err := opts.ValidateHost(h); err != nil {
			return err
		}
	}

	// validate platform-specific settings
	return config.ValidatePlatformConfig()
}

// MaskCredentials masks credentials that are in an URL.
func MaskCredentials(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.User == nil {
		return rawURL
	}
	parsedURL.User = url.UserPassword("xxxxx", "xxxxx")
	return parsedURL.String()
}
