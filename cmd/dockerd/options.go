package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/homedir"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/spf13/pflag"
)

const (
	// DefaultCaFile is the default filename for the CA pem file
	DefaultCaFile = "ca.pem"
	// DefaultKeyFile is the default filename for the key pem file
	DefaultKeyFile = "key.pem"
	// DefaultCertFile is the default filename for the cert pem file
	DefaultCertFile = "cert.pem"
	// FlagTLSVerify is the flag name for the TLS verification option
	FlagTLSVerify = "tlsverify"
	// FlagTLS is the flag name for the TLS option
	FlagTLS = "tls"
	// DefaultTLSValue is the default value used for setting the tls option for tcp connections
	DefaultTLSValue = false
)

var (
	// The configDir (and "DOCKER_CONFIG" environment variable) is now only used
	// for the default location for TLS certificates to secure the daemon API.
	// It is a leftover from when the "docker" and "dockerd" CLI shared the
	// same binary, allowing the DOCKER_CONFIG environment variable to set
	// the location for certificates to be used by both.
	//
	// We need to change this, as there's various issues:
	//
	//   - DOCKER_CONFIG only affects TLS certificates, but does not change the
	//     location for the actual *daemon configuration* (which defaults to
	//     "/etc/docker/daemon.json").
	//   - If no value is set, configDir uses "~/.docker/" as default, but does
	//     not take $XDG_CONFIG_HOME into account (it uses pkg/homedir.Get, which
	//     is not XDG_CONFIG_HOME-aware).
	//   - Using the home directory can be problematic in cases where the CLI and
	//     daemon actually live on the same host; if DOCKER_CONFIG is set to set
	//     the "docker" CLI configuration path (and if the daemon shares that
	//     environment variable, e.g. "sudo -E dockerd"), the daemon may create
	//     the "~/.docker/" directory, but now the directory may be owned by "root".
	//
	// We should:
	//
	//   - deprecate DOCKER_CONFIG for the daemon
	//   - decide where the TLS certs should live by default ("/etc/docker/"?)
	//   - look at "when" (and when _not_) XDG_CONFIG_HOME should be used. Its
	//     needed for rootless, but perhaps could be used for non-rootless(?)
	//   - When changing  the location for TLS config, (ideally) they should
	//     live in a directory separate from "non-sensitive" (configuration-)
	//     files, so that general configuration can be shared (dotfiles repo
	//     etc) separate from "sensitive" config (TLS certificates).
	//
	// TODO(thaJeztah): deprecate DOCKER_CONFIG and re-design daemon config locations. See https://github.com/moby/moby/issues/44640
	configDir       = os.Getenv("DOCKER_CONFIG")
	configFileDir   = ".docker"
	dockerCertPath  = os.Getenv("DOCKER_CERT_PATH")
	dockerTLSVerify = os.Getenv("DOCKER_TLS_VERIFY") != ""
)

type daemonOptions struct {
	configFile   string
	daemonConfig *config.Config
	flags        *pflag.FlagSet
	Debug        bool
	Hosts        []string
	LogLevel     string
	LogFormat    string
	TLS          bool
	TLSVerify    bool
	TLSOptions   *tlsconfig.Options
	Validate     bool
}

// defaultCertPath uses $DOCKER_CONFIG or ~/.docker, and does not look up
// $XDG_CONFIG_HOME. See the comment on configDir above for further details.
func defaultCertPath() string {
	if configDir == "" {
		// Set the default path if DOCKER_CONFIG is not set.
		configDir = filepath.Join(homedir.Get(), configFileDir)
	}
	return configDir
}

// newDaemonOptions returns a new daemonFlags
func newDaemonOptions(config *config.Config) *daemonOptions {
	return &daemonOptions{
		daemonConfig: config,
	}
}

// installFlags adds flags for the common options on the FlagSet
func (o *daemonOptions) installFlags(flags *pflag.FlagSet) {
	if dockerCertPath == "" {
		dockerCertPath = defaultCertPath()
	}

	flags.BoolVarP(&o.Debug, "debug", "D", false, "Enable debug mode")
	flags.BoolVar(&o.Validate, "validate", false, "Validate daemon configuration and exit")
	flags.StringVarP(&o.LogLevel, "log-level", "l", "info", `Set the logging level ("debug"|"info"|"warn"|"error"|"fatal")`)
	flags.StringVar(&o.LogFormat, "log-format", log.TextFormat,
		fmt.Sprintf(`Set the logging format ("%s"|"%s")`, log.TextFormat, log.JSONFormat))
	flags.BoolVar(&o.TLS, FlagTLS, DefaultTLSValue, "Use TLS; implied by --tlsverify")
	flags.BoolVar(&o.TLSVerify, FlagTLSVerify, dockerTLSVerify || DefaultTLSValue, "Use TLS and verify the remote")

	// TODO(thaJeztah): set default TLSOptions in config.New()
	o.TLSOptions = &tlsconfig.Options{}
	tlsOptions := o.TLSOptions
	flags.StringVar(&tlsOptions.CAFile, "tlscacert", filepath.Join(dockerCertPath, DefaultCaFile), "Trust certs signed only by this CA")
	flags.StringVar(&tlsOptions.CertFile, "tlscert", filepath.Join(dockerCertPath, DefaultCertFile), "Path to TLS certificate file")
	flags.StringVar(&tlsOptions.KeyFile, "tlskey", filepath.Join(dockerCertPath, DefaultKeyFile), "Path to TLS key file")

	hostOpt := opts.NewNamedListOptsRef("hosts", &o.Hosts, opts.ValidateHost)
	flags.VarP(hostOpt, "host", "H", "Daemon socket(s) to connect to")
}

// setDefaultOptions sets default values for options after flag parsing is
// complete
func (o *daemonOptions) setDefaultOptions() {
	// Regardless of whether the user sets it to true or false, if they
	// specify --tlsverify at all then we need to turn on TLS
	// TLSVerify can be true even if not set due to DOCKER_TLS_VERIFY env var, so we need
	// to check that here as well
	if o.flags.Changed(FlagTLSVerify) || o.TLSVerify {
		o.TLS = true
	}

	if o.TLS && !o.flags.Changed(FlagTLSVerify) {
		// Enable tls verification unless explicitly disabled
		o.TLSVerify = true
	}

	if !o.TLS {
		o.TLSOptions = nil
	} else {
		o.TLSOptions.InsecureSkipVerify = !o.TLSVerify

		// Reset CertFile and KeyFile to empty string if the user did not specify
		// the respective flags and the respective default files were not found.
		if !o.flags.Changed("tlscert") {
			if _, err := os.Stat(o.TLSOptions.CertFile); os.IsNotExist(err) {
				o.TLSOptions.CertFile = ""
			}
		}
		if !o.flags.Changed("tlskey") {
			if _, err := os.Stat(o.TLSOptions.KeyFile); os.IsNotExist(err) {
				o.TLSOptions.KeyFile = ""
			}
		}
	}
}
