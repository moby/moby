package daemon // import "github.com/docker/docker/daemon"

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/docker/docker/daemon/config"
	"github.com/sirupsen/logrus"
)

// Reload modifies the live daemon configuration from conf.
// conf is assumed to be a validated configuration.
//
// These are the settings that Reload changes:
// - Platform runtime
// - Daemon debug log level
// - Daemon max concurrent downloads
// - Daemon max concurrent uploads
// - Daemon max download attempts
// - Daemon shutdown timeout (in seconds)
// - Cluster discovery (reconfigure and restart)
// - Daemon labels
// - Insecure registries
// - Registry mirrors
// - Daemon live restore
func (daemon *Daemon) Reload(conf *config.Config) (err error) {
	daemon.configStore.Lock()
	attributes := map[string]string{}

	defer func() {
		if err == nil {
			jsonString, _ := json.Marshal(&struct {
				*config.Config
				config.Proxies `json:"proxies"`
			}{
				Config: daemon.configStore,
				Proxies: config.Proxies{
					HTTPProxy:  config.MaskCredentials(daemon.configStore.HTTPProxy),
					HTTPSProxy: config.MaskCredentials(daemon.configStore.HTTPSProxy),
					NoProxy:    config.MaskCredentials(daemon.configStore.NoProxy),
				},
			})
			logrus.Infof("Reloaded configuration: %s", jsonString)
		}
		daemon.configStore.Unlock()
		if err == nil {
			daemon.LogDaemonEventWithAttributes("reload", attributes)
		}
	}()

	// Ideally reloading should be transactional: the reload either completes
	// successfully, or the daemon config and state are left untouched. We use a
	// simplified two-phase commit protocol to achieve this. Any fallible reload
	// operation is split into two phases. The first phase performs all the fallible
	// operations without mutating daemon state and returns a closure: its second
	// phase. The second phase applies the changes to the daemon state. If any
	// first-phase returns an error, the reload transaction is "rolled back" by
	// discarding the second-phase closures.

	type TxnCommitter = func(attributes map[string]string)
	var txns []TxnCommitter
	for _, prepare := range []func(*config.Config) (TxnCommitter, error){
		daemon.reloadPlatform,
		daemon.reloadRegistryConfig,
	} {
		commit, err := prepare(conf)
		if err != nil {
			return err
		}
		txns = append(txns, commit)
	}

	daemon.reloadDebug(conf, attributes)
	daemon.reloadMaxConcurrentDownloadsAndUploads(conf, attributes)
	daemon.reloadMaxDownloadAttempts(conf, attributes)
	daemon.reloadShutdownTimeout(conf, attributes)
	daemon.reloadFeatures(conf, attributes)
	daemon.reloadLabels(conf, attributes)
	daemon.reloadLiveRestore(conf, attributes)
	daemon.reloadNetworkDiagnosticPort(conf, attributes)

	for _, tx := range txns {
		tx(attributes)
	}
	return nil
}

func marshalAttributeSlice(v []string) string {
	if v == nil {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // Should never happen as the input type is fixed.
	}
	return string(b)
}

// reloadDebug updates configuration with Debug option
// and updates the passed attributes
func (daemon *Daemon) reloadDebug(conf *config.Config, attributes map[string]string) {
	// update corresponding configuration
	if conf.IsValueSet("debug") {
		daemon.configStore.Debug = conf.Debug
	}
	// prepare reload event attributes with updatable configurations
	attributes["debug"] = strconv.FormatBool(daemon.configStore.Debug)
}

// reloadMaxConcurrentDownloadsAndUploads updates configuration with max concurrent
// download and upload options and updates the passed attributes
func (daemon *Daemon) reloadMaxConcurrentDownloadsAndUploads(conf *config.Config, attributes map[string]string) {
	// We always "reset" as the cost is lightweight and easy to maintain.
	daemon.configStore.MaxConcurrentDownloads = config.DefaultMaxConcurrentDownloads
	daemon.configStore.MaxConcurrentUploads = config.DefaultMaxConcurrentUploads

	if conf.IsValueSet("max-concurrent-downloads") && conf.MaxConcurrentDownloads != 0 {
		daemon.configStore.MaxConcurrentDownloads = conf.MaxConcurrentDownloads
	}
	if conf.IsValueSet("max-concurrent-uploads") && conf.MaxConcurrentUploads != 0 {
		daemon.configStore.MaxConcurrentUploads = conf.MaxConcurrentUploads
	}
	if daemon.imageService != nil {
		daemon.imageService.UpdateConfig(
			daemon.configStore.MaxConcurrentDownloads,
			daemon.configStore.MaxConcurrentUploads,
		)
	}

	// prepare reload event attributes with updatable configurations
	attributes["max-concurrent-downloads"] = strconv.Itoa(daemon.configStore.MaxConcurrentDownloads)
	attributes["max-concurrent-uploads"] = strconv.Itoa(daemon.configStore.MaxConcurrentUploads)
	logrus.Debug("Reset Max Concurrent Downloads: ", attributes["max-concurrent-downloads"])
	logrus.Debug("Reset Max Concurrent Uploads: ", attributes["max-concurrent-uploads"])
}

// reloadMaxDownloadAttempts updates configuration with max concurrent
// download attempts when a connection is lost and updates the passed attributes
func (daemon *Daemon) reloadMaxDownloadAttempts(conf *config.Config, attributes map[string]string) {
	// We always "reset" as the cost is lightweight and easy to maintain.
	daemon.configStore.MaxDownloadAttempts = config.DefaultDownloadAttempts
	if conf.IsValueSet("max-download-attempts") && conf.MaxDownloadAttempts != 0 {
		daemon.configStore.MaxDownloadAttempts = conf.MaxDownloadAttempts
	}

	// prepare reload event attributes with updatable configurations
	attributes["max-download-attempts"] = strconv.Itoa(daemon.configStore.MaxDownloadAttempts)
	logrus.Debug("Reset Max Download Attempts: ", attributes["max-download-attempts"])
}

// reloadShutdownTimeout updates configuration with daemon shutdown timeout option
// and updates the passed attributes
func (daemon *Daemon) reloadShutdownTimeout(conf *config.Config, attributes map[string]string) {
	// update corresponding configuration
	if conf.IsValueSet("shutdown-timeout") {
		daemon.configStore.ShutdownTimeout = conf.ShutdownTimeout
		logrus.Debugf("Reset Shutdown Timeout: %d", daemon.configStore.ShutdownTimeout)
	}

	// prepare reload event attributes with updatable configurations
	attributes["shutdown-timeout"] = strconv.Itoa(daemon.configStore.ShutdownTimeout)
}

// reloadLabels updates configuration with engine labels
// and updates the passed attributes
func (daemon *Daemon) reloadLabels(conf *config.Config, attributes map[string]string) {
	// update corresponding configuration
	if conf.IsValueSet("labels") {
		daemon.configStore.Labels = conf.Labels
	}

	// prepare reload event attributes with updatable configurations
	attributes["labels"] = marshalAttributeSlice(daemon.configStore.Labels)
}

// reloadRegistryConfig updates the configuration with registry options
// and updates the passed attributes.
func (daemon *Daemon) reloadRegistryConfig(conf *config.Config) (func(map[string]string), error) {
	// Update corresponding configuration.
	opts := daemon.configStore.ServiceOptions

	if conf.IsValueSet("allow-nondistributable-artifacts") {
		opts.AllowNondistributableArtifacts = conf.AllowNondistributableArtifacts
	}
	if conf.IsValueSet("insecure-registries") {
		opts.InsecureRegistries = conf.InsecureRegistries
	}
	if conf.IsValueSet("registry-mirrors") {
		opts.Mirrors = conf.Mirrors
	}

	commit, err := daemon.registryService.ReplaceConfig(opts)
	if err != nil {
		return nil, err
	}

	return func(attributes map[string]string) {
		commit()
		daemon.configStore.ServiceOptions = opts
		// Prepare reload event attributes with updatable configurations.
		attributes["allow-nondistributable-artifacts"] = marshalAttributeSlice(daemon.configStore.AllowNondistributableArtifacts)
		attributes["insecure-registries"] = marshalAttributeSlice(daemon.configStore.InsecureRegistries)
		attributes["registry-mirrors"] = marshalAttributeSlice(daemon.configStore.Mirrors)
	}, nil
}

// reloadLiveRestore updates configuration with live restore option
// and updates the passed attributes
func (daemon *Daemon) reloadLiveRestore(conf *config.Config, attributes map[string]string) {
	// update corresponding configuration
	if conf.IsValueSet("live-restore") {
		daemon.configStore.LiveRestoreEnabled = conf.LiveRestoreEnabled
	}

	// prepare reload event attributes with updatable configurations
	attributes["live-restore"] = strconv.FormatBool(daemon.configStore.LiveRestoreEnabled)
}

// reloadNetworkDiagnosticPort updates the network controller starting the diagnostic if the config is valid
func (daemon *Daemon) reloadNetworkDiagnosticPort(conf *config.Config, attributes map[string]string) {
	if conf == nil || daemon.netController == nil || !conf.IsValueSet("network-diagnostic-port") ||
		conf.NetworkDiagnosticPort < 1 || conf.NetworkDiagnosticPort > 65535 {
		// If there is no config make sure that the diagnostic is off
		if daemon.netController != nil {
			daemon.netController.StopDiagnostic()
		}
		return
	}
	// Enable the network diagnostic if the flag is set with a valid port within the range
	logrus.WithFields(logrus.Fields{"port": conf.NetworkDiagnosticPort, "ip": "127.0.0.1"}).Warn("Starting network diagnostic server")
	daemon.netController.StartDiagnostic(conf.NetworkDiagnosticPort)
}

// reloadFeatures updates configuration with enabled/disabled features
func (daemon *Daemon) reloadFeatures(conf *config.Config, attributes map[string]string) {
	// update corresponding configuration
	// note that we allow features option to be entirely unset
	daemon.configStore.Features = conf.Features

	// prepare reload event attributes with updatable configurations
	attributes["features"] = fmt.Sprintf("%v", daemon.configStore.Features)
}
