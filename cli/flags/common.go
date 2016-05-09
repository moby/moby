package flags

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/go-connections/tlsconfig"
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
	// TLSVerifyKey is the default flag name for the tls verification option
	TLSVerifyKey = "tlsverify"
)

var (
	dockerCertPath  = os.Getenv("DOCKER_CERT_PATH")
	dockerTLSVerify = os.Getenv("DOCKER_TLS_VERIFY") != ""
)

// CommonFlags are flags common to both the client and the daemon.
type CommonFlags struct {
	FlagSet   *flag.FlagSet
	PostParse func()

	Debug      bool
	Hosts      []string
	LogLevel   string
	TLS        bool
	TLSVerify  bool
	TLSOptions *tlsconfig.Options
	TrustKey   string
}

// InitCommonFlags initializes flags common to both client and daemon
func InitCommonFlags() *CommonFlags {
	var commonFlags = &CommonFlags{FlagSet: new(flag.FlagSet)}

	if dockerCertPath == "" {
		dockerCertPath = cliconfig.ConfigDir()
	}

	commonFlags.PostParse = func() { postParseCommon(commonFlags) }

	cmd := commonFlags.FlagSet

	cmd.BoolVar(&commonFlags.Debug, []string{"D", "-debug"}, false, "Enable debug mode")
	cmd.StringVar(&commonFlags.LogLevel, []string{"l", "-log-level"}, "info", "Set the logging level")
	cmd.BoolVar(&commonFlags.TLS, []string{"-tls"}, false, "Use TLS; implied by --tlsverify")
	cmd.BoolVar(&commonFlags.TLSVerify, []string{"-tlsverify"}, dockerTLSVerify, "Use TLS and verify the remote")

	// TODO use flag flag.String([]string{"i", "-identity"}, "", "Path to libtrust key file")

	var tlsOptions tlsconfig.Options
	commonFlags.TLSOptions = &tlsOptions
	cmd.StringVar(&tlsOptions.CAFile, []string{"-tlscacert"}, filepath.Join(dockerCertPath, DefaultCaFile), "Trust certs signed only by this CA")
	cmd.StringVar(&tlsOptions.CertFile, []string{"-tlscert"}, filepath.Join(dockerCertPath, DefaultCertFile), "Path to TLS certificate file")
	cmd.StringVar(&tlsOptions.KeyFile, []string{"-tlskey"}, filepath.Join(dockerCertPath, DefaultKeyFile), "Path to TLS key file")

	cmd.Var(opts.NewNamedListOptsRef("hosts", &commonFlags.Hosts, opts.ValidateHost), []string{"H", "-host"}, "Daemon socket(s) to connect to")
	return commonFlags
}

func postParseCommon(commonFlags *CommonFlags) {
	cmd := commonFlags.FlagSet

	SetDaemonLogLevel(commonFlags.LogLevel)

	// Regardless of whether the user sets it to true or false, if they
	// specify --tlsverify at all then we need to turn on tls
	// TLSVerify can be true even if not set due to DOCKER_TLS_VERIFY env var, so we need
	// to check that here as well
	if cmd.IsSet("-"+TLSVerifyKey) || commonFlags.TLSVerify {
		commonFlags.TLS = true
	}

	if !commonFlags.TLS {
		commonFlags.TLSOptions = nil
	} else {
		tlsOptions := commonFlags.TLSOptions
		tlsOptions.InsecureSkipVerify = !commonFlags.TLSVerify

		// Reset CertFile and KeyFile to empty string if the user did not specify
		// the respective flags and the respective default files were not found.
		if !cmd.IsSet("-tlscert") {
			if _, err := os.Stat(tlsOptions.CertFile); os.IsNotExist(err) {
				tlsOptions.CertFile = ""
			}
		}
		if !cmd.IsSet("-tlskey") {
			if _, err := os.Stat(tlsOptions.KeyFile); os.IsNotExist(err) {
				tlsOptions.KeyFile = ""
			}
		}
	}
}

// SetDaemonLogLevel sets the logrus logging level
// TODO: this is a bad name, it applies to the client as well.
func SetDaemonLogLevel(logLevel string) {
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
