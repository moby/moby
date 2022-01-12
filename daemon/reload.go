package daemon // import "github.com/docker/docker/daemon"

import (
	"encoding/json"
	"fmt"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/discovery"
	"github.com/sirupsen/logrus"
)

// Reload reads configuration changes and modifies the
// daemon according to those changes.
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
				config.ProxyConfig
			}{
				Config: daemon.configStore,
				ProxyConfig: config.ProxyConfig{
					HTTPProxy:  config.MaskCredentials(daemon.configStore.HTTPProxy),
					HTTPSProxy: config.MaskCredentials(daemon.configStore.HTTPSProxy),
					NoProxy:    config.MaskCredentials(daemon.configStore.NoProxy),
				},
			})
			logrus.Infof("Reloaded configuration: %s", jsonString)
		}

		// we're unlocking here, because
		// LogDaemonEventWithAttributes() -> SystemInfo() -> GetAllRuntimes()
		// holds that lock too.
		daemon.configStore.Unlock()
		if err == nil {
			daemon.LogDaemonEventWithAttributes("reload", attributes)
		}
	}()

	if err := daemon.reloadPlatform(conf, attributes); err != nil {
		return err
	}
	daemon.reloadDebug(conf, attributes)
	daemon.reloadMaxConcurrentDownloadsAndUploads(conf, attributes)
	if err := daemon.reloadMaxDownloadAttempts(conf, attributes); err != nil {
		return err
	}
	daemon.reloadShutdownTimeout(conf, attributes)
	daemon.reloadFeatures(conf, attributes)

	if err := daemon.reloadClusterDiscovery(conf, attributes); err != nil {
		return err
	}
	if err := daemon.reloadLabels(conf, attributes); err != nil {
		return err
	}
	if err := daemon.reloadAllowNondistributableArtifacts(conf, attributes); err != nil {
		return err
	}
	if err := daemon.reloadInsecureRegistries(conf, attributes); err != nil {
		return err
	}
	if err := daemon.reloadRegistryMirrors(conf, attributes); err != nil {
		return err
	}
	if err := daemon.reloadLiveRestore(conf, attributes); err != nil {
		return err
	}
	return daemon.reloadNetworkDiagnosticPort(conf, attributes)
}

// reloadDebug updates configuration with Debug option
// and updates the passed attributes
func (daemon *Daemon) reloadDebug(conf *config.Config, attributes map[string]string) {
	// update corresponding configuration
	if conf.IsValueSet("debug") {
		daemon.configStore.Debug = conf.Debug
	}
	// prepare reload event attributes with updatable configurations
	attributes["debug"] = fmt.Sprintf("%t", daemon.configStore.Debug)
}

// reloadMaxConcurrentDownloadsAndUploads updates configuration with max concurrent
// download and upload options and updates the passed attributes
func (daemon *Daemon) reloadMaxConcurrentDownloadsAndUploads(conf *config.Config, attributes map[string]string) {
	// If no value is set for max-concurrent-downloads we assume it is the default value
	// We always "reset" as the cost is lightweight and easy to maintain.
	maxConcurrentDownloads := config.DefaultMaxConcurrentDownloads
	if conf.IsValueSet("max-concurrent-downloads") && conf.MaxConcurrentDownloads != nil {
		maxConcurrentDownloads = *conf.MaxConcurrentDownloads
	}
	daemon.configStore.MaxConcurrentDownloads = &maxConcurrentDownloads
	logrus.Debugf("Reset Max Concurrent Downloads: %d", *daemon.configStore.MaxConcurrentDownloads)

	// If no value is set for max-concurrent-upload we assume it is the default value
	// We always "reset" as the cost is lightweight and easy to maintain.
	maxConcurrentUploads := config.DefaultMaxConcurrentUploads
	if conf.IsValueSet("max-concurrent-uploads") && conf.MaxConcurrentUploads != nil {
		maxConcurrentUploads = *conf.MaxConcurrentUploads
	}
	daemon.configStore.MaxConcurrentUploads = &maxConcurrentUploads
	logrus.Debugf("Reset Max Concurrent Uploads: %d", *daemon.configStore.MaxConcurrentUploads)

	if daemon.imageService != nil {
		daemon.imageService.UpdateConfig(&maxConcurrentDownloads, &maxConcurrentUploads)
	}

	// prepare reload event attributes with updatable configurations
	attributes["max-concurrent-downloads"] = fmt.Sprintf("%d", *daemon.configStore.MaxConcurrentDownloads)
	// prepare reload event attributes with updatable configurations
	attributes["max-concurrent-uploads"] = fmt.Sprintf("%d", *daemon.configStore.MaxConcurrentUploads)
}

// reloadMaxDownloadAttempts updates configuration with max concurrent
// download attempts when a connection is lost and updates the passed attributes
func (daemon *Daemon) reloadMaxDownloadAttempts(conf *config.Config, attributes map[string]string) error {
	if err := config.ValidateMaxDownloadAttempts(conf); err != nil {
		return err
	}

	// If no value is set for max-download-attempts we assume it is the default value
	// We always "reset" as the cost is lightweight and easy to maintain.
	maxDownloadAttempts := config.DefaultDownloadAttempts
	if conf.IsValueSet("max-download-attempts") && conf.MaxDownloadAttempts != nil {
		maxDownloadAttempts = *conf.MaxDownloadAttempts
	}
	daemon.configStore.MaxDownloadAttempts = &maxDownloadAttempts
	logrus.Debugf("Reset Max Download Attempts: %d", *daemon.configStore.MaxDownloadAttempts)

	// prepare reload event attributes with updatable configurations
	attributes["max-download-attempts"] = fmt.Sprintf("%d", *daemon.configStore.MaxDownloadAttempts)
	return nil
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
	attributes["shutdown-timeout"] = fmt.Sprintf("%d", daemon.configStore.ShutdownTimeout)
}

// reloadClusterDiscovery updates configuration with cluster discovery options
// and updates the passed attributes
func (daemon *Daemon) reloadClusterDiscovery(conf *config.Config, attributes map[string]string) (err error) {
	defer func() {
		// prepare reload event attributes with updatable configurations
		attributes["cluster-store"] = conf.ClusterStore
		attributes["cluster-advertise"] = conf.ClusterAdvertise

		attributes["cluster-store-opts"] = "{}"
		if daemon.configStore.ClusterOpts != nil {
			opts, err2 := json.Marshal(conf.ClusterOpts)
			if err != nil {
				err = err2
			}
			attributes["cluster-store-opts"] = string(opts)
		}
	}()

	newAdvertise := conf.ClusterAdvertise
	newClusterStore := daemon.configStore.ClusterStore
	if conf.IsValueSet("cluster-advertise") {
		if conf.IsValueSet("cluster-store") {
			newClusterStore = conf.ClusterStore
		}
		newAdvertise, err = config.ParseClusterAdvertiseSettings(newClusterStore, conf.ClusterAdvertise)
		if err != nil && err != discovery.ErrDiscoveryDisabled {
			return err
		}
	}

	if daemon.clusterProvider != nil {
		if err := conf.IsSwarmCompatible(); err != nil {
			return err
		}
	}

	// check discovery modifications
	if !config.ModifiedDiscoverySettings(daemon.configStore, newClusterStore, newAdvertise, conf.ClusterOpts) {
		return nil
	}

	// enable discovery for the first time if it was not previously enabled
	if daemon.discoveryWatcher == nil {
		discoveryWatcher, err := discovery.Init(newClusterStore, newAdvertise, conf.ClusterOpts)
		if err != nil {
			return fmt.Errorf("failed to initialize discovery: %v", err)
		}
		daemon.discoveryWatcher = discoveryWatcher
	} else if err == discovery.ErrDiscoveryDisabled {
		// disable discovery if it was previously enabled and it's disabled now
		daemon.discoveryWatcher.Stop()
	} else if err = daemon.discoveryWatcher.Reload(conf.ClusterStore, newAdvertise, conf.ClusterOpts); err != nil {
		// reload discovery
		return err
	}

	daemon.configStore.ClusterStore = newClusterStore
	daemon.configStore.ClusterOpts = conf.ClusterOpts
	daemon.configStore.ClusterAdvertise = newAdvertise

	if daemon.netController == nil {
		return nil
	}
	netOptions, err := daemon.networkOptions(daemon.configStore, daemon.PluginStore, nil)
	if err != nil {
		logrus.WithError(err).Warn("failed to get options with network controller")
		return nil
	}
	err = daemon.netController.ReloadConfiguration(netOptions...)
	if err != nil {
		logrus.Warnf("Failed to reload configuration with network controller: %v", err)
	}
	return nil
}

// reloadLabels updates configuration with engine labels
// and updates the passed attributes
func (daemon *Daemon) reloadLabels(conf *config.Config, attributes map[string]string) error {
	// update corresponding configuration
	if conf.IsValueSet("labels") {
		daemon.configStore.Labels = conf.Labels
	}

	// prepare reload event attributes with updatable configurations
	if daemon.configStore.Labels != nil {
		labels, err := json.Marshal(daemon.configStore.Labels)
		if err != nil {
			return err
		}
		attributes["labels"] = string(labels)
	} else {
		attributes["labels"] = "[]"
	}

	return nil
}

// reloadAllowNondistributableArtifacts updates the configuration with allow-nondistributable-artifacts options
// and updates the passed attributes.
func (daemon *Daemon) reloadAllowNondistributableArtifacts(conf *config.Config, attributes map[string]string) error {
	// Update corresponding configuration.
	if conf.IsValueSet("allow-nondistributable-artifacts") {
		daemon.configStore.AllowNondistributableArtifacts = conf.AllowNondistributableArtifacts
		if err := daemon.RegistryService.LoadAllowNondistributableArtifacts(conf.AllowNondistributableArtifacts); err != nil {
			return err
		}
	}

	// Prepare reload event attributes with updatable configurations.
	if daemon.configStore.AllowNondistributableArtifacts != nil {
		v, err := json.Marshal(daemon.configStore.AllowNondistributableArtifacts)
		if err != nil {
			return err
		}
		attributes["allow-nondistributable-artifacts"] = string(v)
	} else {
		attributes["allow-nondistributable-artifacts"] = "[]"
	}

	return nil
}

// reloadInsecureRegistries updates configuration with insecure registry option
// and updates the passed attributes
func (daemon *Daemon) reloadInsecureRegistries(conf *config.Config, attributes map[string]string) error {
	// update corresponding configuration
	if conf.IsValueSet("insecure-registries") {
		daemon.configStore.InsecureRegistries = conf.InsecureRegistries
		if err := daemon.RegistryService.LoadInsecureRegistries(conf.InsecureRegistries); err != nil {
			return err
		}
	}

	// prepare reload event attributes with updatable configurations
	if daemon.configStore.InsecureRegistries != nil {
		insecureRegistries, err := json.Marshal(daemon.configStore.InsecureRegistries)
		if err != nil {
			return err
		}
		attributes["insecure-registries"] = string(insecureRegistries)
	} else {
		attributes["insecure-registries"] = "[]"
	}

	return nil
}

// reloadRegistryMirrors updates configuration with registry mirror options
// and updates the passed attributes
func (daemon *Daemon) reloadRegistryMirrors(conf *config.Config, attributes map[string]string) error {
	// update corresponding configuration
	if conf.IsValueSet("registry-mirrors") {
		daemon.configStore.Mirrors = conf.Mirrors
		if err := daemon.RegistryService.LoadMirrors(conf.Mirrors); err != nil {
			return err
		}
	}

	// prepare reload event attributes with updatable configurations
	if daemon.configStore.Mirrors != nil {
		mirrors, err := json.Marshal(daemon.configStore.Mirrors)
		if err != nil {
			return err
		}
		attributes["registry-mirrors"] = string(mirrors)
	} else {
		attributes["registry-mirrors"] = "[]"
	}

	return nil
}

// reloadLiveRestore updates configuration with live restore option
// and updates the passed attributes
func (daemon *Daemon) reloadLiveRestore(conf *config.Config, attributes map[string]string) error {
	// update corresponding configuration
	if conf.IsValueSet("live-restore") {
		daemon.configStore.LiveRestoreEnabled = conf.LiveRestoreEnabled
	}

	// prepare reload event attributes with updatable configurations
	attributes["live-restore"] = fmt.Sprintf("%t", daemon.configStore.LiveRestoreEnabled)
	return nil
}

// reloadNetworkDiagnosticPort updates the network controller starting the diagnostic if the config is valid
func (daemon *Daemon) reloadNetworkDiagnosticPort(conf *config.Config, attributes map[string]string) error {
	if conf == nil || daemon.netController == nil || !conf.IsValueSet("network-diagnostic-port") ||
		conf.NetworkDiagnosticPort < 1 || conf.NetworkDiagnosticPort > 65535 {
		// If there is no config make sure that the diagnostic is off
		if daemon.netController != nil {
			daemon.netController.StopDiagnostic()
		}
		return nil
	}
	// Enable the network diagnostic if the flag is set with a valid port within the range
	logrus.WithFields(logrus.Fields{"port": conf.NetworkDiagnosticPort, "ip": "127.0.0.1"}).Warn("Starting network diagnostic server")
	daemon.netController.StartDiagnostic(conf.NetworkDiagnosticPort)

	return nil
}

// reloadFeatures updates configuration with enabled/disabled features
func (daemon *Daemon) reloadFeatures(conf *config.Config, attributes map[string]string) {
	// update corresponding configuration
	// note that we allow features option to be entirely unset
	daemon.configStore.Features = conf.Features

	// prepare reload event attributes with updatable configurations
	attributes["features"] = fmt.Sprintf("%v", daemon.configStore.Features)
}
