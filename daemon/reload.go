package daemon // import "github.com/docker/docker/daemon"

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/copystructure"
	"github.com/sirupsen/logrus"

	"github.com/docker/docker/daemon/config"
)

// reloadTxn is used to defer side effects of a config reload.
type reloadTxn struct {
	onCommit, onRollback []func() error
}

// OnCommit defers a function to be called when a config reload is being finalized.
// The error returned from cb is purely informational.
func (tx *reloadTxn) OnCommit(cb func() error) {
	tx.onCommit = append(tx.onCommit, cb)
}

// OnRollback defers a function to be called when a config reload is aborted.
// The error returned from cb is purely informational.
func (tx *reloadTxn) OnRollback(cb func() error) {
	tx.onCommit = append(tx.onRollback, cb)
}

func (tx *reloadTxn) run(cbs []func() error) error {
	tx.onCommit = nil
	tx.onRollback = nil

	var res *multierror.Error
	for _, cb := range cbs {
		res = multierror.Append(res, cb())
	}
	return res.ErrorOrNil()
}

// Commit calls all functions registered with OnCommit.
// Any errors returned by the functions are collated into a
// *github.com/hashicorp/go-multierror.Error value.
func (tx *reloadTxn) Commit() error {
	return tx.run(tx.onCommit)
}

// Rollback calls all functions registered with OnRollback.
// Any errors returned by the functions are collated into a
// *github.com/hashicorp/go-multierror.Error value.
func (tx *reloadTxn) Rollback() error {
	return tx.run(tx.onRollback)
}

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
func (daemon *Daemon) Reload(conf *config.Config) error {
	daemon.configReload.Lock()
	defer daemon.configReload.Unlock()
	copied, err := copystructure.Copy(daemon.config())
	if err != nil {
		return err
	}
	newCfg := copied.(*config.Config)

	attributes := map[string]string{}

	// Ideally reloading should be transactional: the reload either completes
	// successfully, or the daemon config and state are left untouched. We use a
	// two-phase commit protocol to achieve this. Any fallible reload operation is
	// split into two phases. The first phase performs all the fallible operations
	// and mutates the newCfg copy. The second phase atomically swaps newCfg into
	// the live daemon configuration and executes any commit functions the first
	// phase registered to apply the side effects. If any first-phase returns an
	// error, the reload transaction is rolled back by discarding newCfg and
	// executing any registered rollback functions.

	var txn reloadTxn
	for _, reload := range []func(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error{
		daemon.reloadPlatform,
		daemon.reloadDebug,
		daemon.reloadMaxConcurrentDownloadsAndUploads,
		daemon.reloadMaxDownloadAttempts,
		daemon.reloadShutdownTimeout,
		daemon.reloadFeatures,
		daemon.reloadLabels,
		daemon.reloadRegistryConfig,
		daemon.reloadLiveRestore,
		daemon.reloadNetworkDiagnosticPort,
	} {
		if err := reload(&txn, newCfg, conf, attributes); err != nil {
			if rollbackErr := txn.Rollback(); rollbackErr != nil {
				return multierror.Append(nil, err, rollbackErr)
			}
			return err
		}
	}

	jsonString, _ := json.Marshal(&struct {
		*config.Config
		config.Proxies `json:"proxies"`
	}{
		Config: newCfg,
		Proxies: config.Proxies{
			HTTPProxy:  config.MaskCredentials(newCfg.HTTPProxy),
			HTTPSProxy: config.MaskCredentials(newCfg.HTTPSProxy),
			NoProxy:    config.MaskCredentials(newCfg.NoProxy),
		},
	})
	logrus.Infof("Reloaded configuration: %s", jsonString)
	daemon.configStore.Store(newCfg)
	daemon.LogDaemonEventWithAttributes("reload", attributes)
	return txn.Commit()
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
func (daemon *Daemon) reloadDebug(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error {
	// update corresponding configuration
	if conf.IsValueSet("debug") {
		newCfg.Debug = conf.Debug
	}
	// prepare reload event attributes with updatable configurations
	attributes["debug"] = strconv.FormatBool(newCfg.Debug)
	return nil
}

// reloadMaxConcurrentDownloadsAndUploads updates configuration with max concurrent
// download and upload options and updates the passed attributes
func (daemon *Daemon) reloadMaxConcurrentDownloadsAndUploads(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error {
	// We always "reset" as the cost is lightweight and easy to maintain.
	newCfg.MaxConcurrentDownloads = config.DefaultMaxConcurrentDownloads
	newCfg.MaxConcurrentUploads = config.DefaultMaxConcurrentUploads

	if conf.IsValueSet("max-concurrent-downloads") && conf.MaxConcurrentDownloads != 0 {
		newCfg.MaxConcurrentDownloads = conf.MaxConcurrentDownloads
	}
	if conf.IsValueSet("max-concurrent-uploads") && conf.MaxConcurrentUploads != 0 {
		newCfg.MaxConcurrentUploads = conf.MaxConcurrentUploads
	}
	txn.OnCommit(func() error {
		if daemon.imageService != nil {
			daemon.imageService.UpdateConfig(
				newCfg.MaxConcurrentDownloads,
				newCfg.MaxConcurrentUploads,
			)
		}
		return nil
	})

	// prepare reload event attributes with updatable configurations
	attributes["max-concurrent-downloads"] = strconv.Itoa(newCfg.MaxConcurrentDownloads)
	attributes["max-concurrent-uploads"] = strconv.Itoa(newCfg.MaxConcurrentUploads)
	logrus.Debug("Reset Max Concurrent Downloads: ", attributes["max-concurrent-downloads"])
	logrus.Debug("Reset Max Concurrent Uploads: ", attributes["max-concurrent-uploads"])
	return nil
}

// reloadMaxDownloadAttempts updates configuration with max concurrent
// download attempts when a connection is lost and updates the passed attributes
func (daemon *Daemon) reloadMaxDownloadAttempts(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error {
	// We always "reset" as the cost is lightweight and easy to maintain.
	newCfg.MaxDownloadAttempts = config.DefaultDownloadAttempts
	if conf.IsValueSet("max-download-attempts") && conf.MaxDownloadAttempts != 0 {
		newCfg.MaxDownloadAttempts = conf.MaxDownloadAttempts
	}

	// prepare reload event attributes with updatable configurations
	attributes["max-download-attempts"] = strconv.Itoa(newCfg.MaxDownloadAttempts)
	logrus.Debug("Reset Max Download Attempts: ", attributes["max-download-attempts"])
	return nil
}

// reloadShutdownTimeout updates configuration with daemon shutdown timeout option
// and updates the passed attributes
func (daemon *Daemon) reloadShutdownTimeout(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error {
	// update corresponding configuration
	if conf.IsValueSet("shutdown-timeout") {
		newCfg.ShutdownTimeout = conf.ShutdownTimeout
		logrus.Debugf("Reset Shutdown Timeout: %d", newCfg.ShutdownTimeout)
	}

	// prepare reload event attributes with updatable configurations
	attributes["shutdown-timeout"] = strconv.Itoa(newCfg.ShutdownTimeout)
	return nil
}

// reloadLabels updates configuration with engine labels
// and updates the passed attributes
func (daemon *Daemon) reloadLabels(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error {
	// update corresponding configuration
	if conf.IsValueSet("labels") {
		newCfg.Labels = conf.Labels
	}

	// prepare reload event attributes with updatable configurations
	attributes["labels"] = marshalAttributeSlice(newCfg.Labels)
	return nil
}

// reloadRegistryConfig updates the configuration with registry options
// and updates the passed attributes.
func (daemon *Daemon) reloadRegistryConfig(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error {
	// Update corresponding configuration.
	if conf.IsValueSet("allow-nondistributable-artifacts") {
		newCfg.ServiceOptions.AllowNondistributableArtifacts = conf.AllowNondistributableArtifacts
	}
	if conf.IsValueSet("insecure-registries") {
		newCfg.ServiceOptions.InsecureRegistries = conf.InsecureRegistries
	}
	if conf.IsValueSet("registry-mirrors") {
		newCfg.ServiceOptions.Mirrors = conf.Mirrors
	}

	commit, err := daemon.registryService.ReplaceConfig(newCfg.ServiceOptions)
	if err != nil {
		return err
	}
	txn.OnCommit(func() error { commit(); return nil })

	attributes["allow-nondistributable-artifacts"] = marshalAttributeSlice(newCfg.ServiceOptions.AllowNondistributableArtifacts)
	attributes["insecure-registries"] = marshalAttributeSlice(newCfg.ServiceOptions.InsecureRegistries)
	attributes["registry-mirrors"] = marshalAttributeSlice(newCfg.ServiceOptions.Mirrors)

	return nil
}

// reloadLiveRestore updates configuration with live restore option
// and updates the passed attributes
func (daemon *Daemon) reloadLiveRestore(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error {
	// update corresponding configuration
	if conf.IsValueSet("live-restore") {
		newCfg.LiveRestoreEnabled = conf.LiveRestoreEnabled
	}

	// prepare reload event attributes with updatable configurations
	attributes["live-restore"] = strconv.FormatBool(newCfg.LiveRestoreEnabled)
	return nil
}

// reloadNetworkDiagnosticPort updates the network controller starting the diagnostic if the config is valid
func (daemon *Daemon) reloadNetworkDiagnosticPort(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error {
	txn.OnCommit(func() error {
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
	})
	return nil
}

// reloadFeatures updates configuration with enabled/disabled features
func (daemon *Daemon) reloadFeatures(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error {
	// update corresponding configuration
	// note that we allow features option to be entirely unset
	newCfg.Features = conf.Features

	// prepare reload event attributes with updatable configurations
	attributes["features"] = fmt.Sprintf("%v", newCfg.Features)
	return nil
}
