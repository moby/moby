package main

import (
	"os"
	"path/filepath"

	cliconfig "github.com/docker/docker/cli/config"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/opts"
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
	TLS          bool
	TLSVerify    bool
	TLSOptions   *tlsconfig.Options
	Validate     bool
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
		// cliconfig.Dir returns $DOCKER_CONFIG or ~/.docker.
		// cliconfig.Dir does not look up $XDG_CONFIG_HOME
		dockerCertPath = cliconfig.Dir()
	}

	flags.BoolVarP(&o.Debug, "debug", "D", false, "Enable debug mode")
	flags.BoolVar(&o.Validate, "validate", false, "Validate daemon configuration and exit")
	flags.StringVarP(&o.LogLevel, "log-level", "l", "info", `Set the logging level ("debug"|"info"|"warn"|"error"|"fatal")`)
	flags.BoolVar(&o.TLS, FlagTLS, DefaultTLSValue, "Use TLS; implied by --tlsverify")
	flags.BoolVar(&o.TLSVerify, FlagTLSVerify, dockerTLSVerify || DefaultTLSValue, "Use TLS and verify the remote")

	// TODO use flag flags.String("identity"}, "i", "", "Path to libtrust key file")

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
