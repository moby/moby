package flags

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	cliconfig "github.com/docker/docker/cli/config"
	"github.com/docker/docker/opts"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/spf13/pflag"
)

const (
	// DefaultTrustKeyFile is the default filename for the trust key
	DefaultTrustKeyFile = "key.json"
	// DefaultCaFile is the default filename for the CA pem file
	DefaultCaFile = "ca.pem"
	// DefaultKeyFile is the default filename for the key pem file
	DefaultKeyFile = "key.pem"
	// DefaultCertFile is the default filename for the cert pem file
	DefaultCertFile = "cert.pem"
	// FlagTLSVerify is the flag name for the TLS verification option
	FlagTLSVerify = "tlsverify"
)

var (
	dockerCertPath  = os.Getenv("DOCKER_CERT_PATH")
	dockerTLSVerify = os.Getenv("DOCKER_TLS_VERIFY") != ""
)

// CommonOptions are options common to both the client and the daemon.
type CommonOptions struct {
	Debug      bool
	Hosts      []string
	LogLevel   string
	TLS        bool
	TLSVerify  bool
	TLSOptions *tlsconfig.Options
	TrustKey   string
}

// NewCommonOptions returns a new CommonOptions
func NewCommonOptions() *CommonOptions {
	return &CommonOptions{}
}

// InstallFlags adds flags for the common options on the FlagSet
func (commonOpts *CommonOptions) InstallFlags(flags *pflag.FlagSet) {
	if dockerCertPath == "" {
		dockerCertPath = cliconfig.Dir()
	}

	flags.BoolVarP(&commonOpts.Debug, "debug", "D", false, "Enable debug mode")
	flags.StringVarP(&commonOpts.LogLevel, "log-level", "l", "info", "Set the logging level (\"debug\", \"info\", \"warn\", \"error\", \"fatal\")")
	flags.BoolVar(&commonOpts.TLS, "tls", false, "Use TLS; implied by --tlsverify")
	flags.BoolVar(&commonOpts.TLSVerify, FlagTLSVerify, dockerTLSVerify, "Use TLS and verify the remote")

	// TODO use flag flags.String("identity"}, "i", "", "Path to libtrust key file")

	commonOpts.TLSOptions = &tlsconfig.Options{}
	tlsOptions := commonOpts.TLSOptions
	flags.StringVar(&tlsOptions.CAFile, "tlscacert", filepath.Join(dockerCertPath, DefaultCaFile), "Trust certs signed only by this CA")
	flags.StringVar(&tlsOptions.CertFile, "tlscert", filepath.Join(dockerCertPath, DefaultCertFile), "Path to TLS certificate file")
	flags.StringVar(&tlsOptions.KeyFile, "tlskey", filepath.Join(dockerCertPath, DefaultKeyFile), "Path to TLS key file")

	hostOpt := opts.NewNamedListOptsRef("hosts", &commonOpts.Hosts, opts.ValidateHost)
	flags.VarP(hostOpt, "host", "H", "Daemon socket(s) to connect to")
}

// SetDefaultOptions sets default values for options after flag parsing is
// complete
func (commonOpts *CommonOptions) SetDefaultOptions(flags *pflag.FlagSet) {
	// Regardless of whether the user sets it to true or false, if they
	// specify --tlsverify at all then we need to turn on TLS
	// TLSVerify can be true even if not set due to DOCKER_TLS_VERIFY env var, so we need
	// to check that here as well
	if flags.Changed(FlagTLSVerify) || commonOpts.TLSVerify {
		commonOpts.TLS = true
	}

	if !commonOpts.TLS {
		commonOpts.TLSOptions = nil
	} else {
		tlsOptions := commonOpts.TLSOptions
		tlsOptions.InsecureSkipVerify = !commonOpts.TLSVerify

		// Reset CertFile and KeyFile to empty string if the user did not specify
		// the respective flags and the respective default files were not found.
		if !flags.Changed("tlscert") {
			if _, err := os.Stat(tlsOptions.CertFile); os.IsNotExist(err) {
				tlsOptions.CertFile = ""
			}
		}
		if !flags.Changed("tlskey") {
			if _, err := os.Stat(tlsOptions.KeyFile); os.IsNotExist(err) {
				tlsOptions.KeyFile = ""
			}
		}
	}
}

// SetLogLevel sets the logrus logging level
func SetLogLevel(logLevel string) {
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to parse logging level: %s\n", logLevel)
			os.Exit(1)
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}
