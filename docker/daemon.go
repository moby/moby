// +build daemon

package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/uuid"
	apiserver "github.com/docker/docker/api/server"
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/build"
	"github.com/docker/docker/api/server/router/container"
	"github.com/docker/docker/api/server/router/image"
	"github.com/docker/docker/api/server/router/network"
	systemrouter "github.com/docker/docker/api/server/router/system"
	"github.com/docker/docker/api/server/router/volume"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/listeners"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/pidfile"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
	"github.com/docker/go-connections/tlsconfig"
)

const (
	daemonUsage          = "       docker daemon [ --help | ... ]\n"
	daemonConfigFileFlag = "-config-file"
)

var (
	daemonCli cli.Handler = NewDaemonCli()
)

// DaemonCli represents the daemon CLI.
type DaemonCli struct {
	*daemon.Config
	flags *flag.FlagSet
}

func presentInHelp(usage string) string { return usage }
func absentFromHelp(string) string      { return "" }

// NewDaemonCli returns a pre-configured daemon CLI
func NewDaemonCli() *DaemonCli {
	daemonFlags := cli.Subcmd("daemon", nil, "Enable daemon mode", true)

	// TODO(tiborvass): remove InstallFlags?
	daemonConfig := new(daemon.Config)
	daemonConfig.LogConfig.Config = make(map[string]string)
	daemonConfig.ClusterOpts = make(map[string]string)

	if runtime.GOOS != "linux" {
		daemonConfig.V2Only = true
	}

	daemonConfig.InstallFlags(daemonFlags, presentInHelp)
	daemonConfig.InstallFlags(flag.CommandLine, absentFromHelp)
	daemonFlags.Require(flag.Exact, 0)

	return &DaemonCli{
		Config: daemonConfig,
		flags:  daemonFlags,
	}
}

func migrateKey() (err error) {
	// Migrate trust key if exists at ~/.docker/key.json and owned by current user
	oldPath := filepath.Join(cliconfig.ConfigDir(), defaultTrustKeyFile)
	newPath := filepath.Join(getDaemonConfDir(), defaultTrustKeyFile)
	if _, statErr := os.Stat(newPath); os.IsNotExist(statErr) && currentUserIsOwner(oldPath) {
		defer func() {
			// Ensure old path is removed if no error occurred
			if err == nil {
				err = os.Remove(oldPath)
			} else {
				logrus.Warnf("Key migration failed, key file not removed at %s", oldPath)
				os.Remove(newPath)
			}
		}()

		if err := system.MkdirAll(getDaemonConfDir(), os.FileMode(0644)); err != nil {
			return fmt.Errorf("Unable to create daemon configuration directory: %s", err)
		}

		newFile, err := os.OpenFile(newPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("error creating key file %q: %s", newPath, err)
		}
		defer newFile.Close()

		oldFile, err := os.Open(oldPath)
		if err != nil {
			return fmt.Errorf("error opening key file %q: %s", oldPath, err)
		}
		defer oldFile.Close()

		if _, err := io.Copy(newFile, oldFile); err != nil {
			return fmt.Errorf("error copying key: %s", err)
		}

		logrus.Infof("Migrated key from %s to %s", oldPath, newPath)
	}

	return nil
}

func getGlobalFlag() (globalFlag *flag.Flag) {
	defer func() {
		if x := recover(); x != nil {
			switch f := x.(type) {
			case *flag.Flag:
				globalFlag = f
			default:
				panic(x)
			}
		}
	}()
	visitor := func(f *flag.Flag) { panic(f) }
	commonFlags.FlagSet.Visit(visitor)
	clientFlags.FlagSet.Visit(visitor)
	return
}

// CmdDaemon is the daemon command, called the raw arguments after `docker daemon`.
func (cli *DaemonCli) CmdDaemon(args ...string) error {
	// warn from uuid package when running the daemon
	uuid.Loggerf = logrus.Warnf

	if !commonFlags.FlagSet.IsEmpty() || !clientFlags.FlagSet.IsEmpty() {
		// deny `docker -D daemon`
		illegalFlag := getGlobalFlag()
		fmt.Fprintf(os.Stderr, "invalid flag '-%s'.\nSee 'docker daemon --help'.\n", illegalFlag.Names[0])
		os.Exit(1)
	} else {
		// allow new form `docker daemon -D`
		flag.Merge(cli.flags, commonFlags.FlagSet)
	}

	configFile := cli.flags.String([]string{daemonConfigFileFlag}, defaultDaemonConfigFile, "Daemon configuration file")

	cli.flags.ParseFlags(args, true)
	commonFlags.PostParse()

	if commonFlags.TrustKey == "" {
		commonFlags.TrustKey = filepath.Join(getDaemonConfDir(), defaultTrustKeyFile)
	}
	cliConfig, err := loadDaemonCliConfig(cli.Config, cli.flags, commonFlags, *configFile)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	cli.Config = cliConfig

	if cli.Config.Debug {
		utils.EnableDebug()
	}

	if utils.ExperimentalBuild() {
		logrus.Warn("Running experimental build")
	}

	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: jsonlog.RFC3339NanoFixed,
		DisableColors:   cli.Config.RawLogs,
	})

	if err := setDefaultUmask(); err != nil {
		logrus.Fatalf("Failed to set umask: %v", err)
	}

	if len(cli.LogConfig.Config) > 0 {
		if err := logger.ValidateLogOpts(cli.LogConfig.Type, cli.LogConfig.Config); err != nil {
			logrus.Fatalf("Failed to set log opts: %v", err)
		}
	}

	var pfile *pidfile.PIDFile
	if cli.Pidfile != "" {
		pf, err := pidfile.New(cli.Pidfile)
		if err != nil {
			logrus.Fatalf("Error starting daemon: %v", err)
		}
		pfile = pf
		defer func() {
			if err := pfile.Remove(); err != nil {
				logrus.Error(err)
			}
		}()
	}

	serverConfig := &apiserver.Config{
		AuthorizationPluginNames: cli.Config.AuthorizationPlugins,
		Logging:                  true,
		SocketGroup:              cli.Config.SocketGroup,
		Version:                  dockerversion.Version,
	}
	serverConfig = setPlatformServerConfig(serverConfig, cli.Config)

	if cli.Config.TLS {
		tlsOptions := tlsconfig.Options{
			CAFile:   cli.Config.CommonTLSOptions.CAFile,
			CertFile: cli.Config.CommonTLSOptions.CertFile,
			KeyFile:  cli.Config.CommonTLSOptions.KeyFile,
		}

		if cli.Config.TLSVerify {
			// server requires and verifies client's certificate
			tlsOptions.ClientAuth = tls.RequireAndVerifyClientCert
		}
		tlsConfig, err := tlsconfig.Server(tlsOptions)
		if err != nil {
			logrus.Fatal(err)
		}
		serverConfig.TLSConfig = tlsConfig
	}

	if len(cli.Config.Hosts) == 0 {
		cli.Config.Hosts = make([]string, 1)
	}

	api := apiserver.New(serverConfig)

	for i := 0; i < len(cli.Config.Hosts); i++ {
		var err error
		if cli.Config.Hosts[i], err = opts.ParseHost(cli.Config.TLS, cli.Config.Hosts[i]); err != nil {
			logrus.Fatalf("error parsing -H %s : %v", cli.Config.Hosts[i], err)
		}

		protoAddr := cli.Config.Hosts[i]
		protoAddrParts := strings.SplitN(protoAddr, "://", 2)
		if len(protoAddrParts) != 2 {
			logrus.Fatalf("bad format %s, expected PROTO://ADDR", protoAddr)
		}
		l, err := listeners.Init(protoAddrParts[0], protoAddrParts[1], serverConfig.SocketGroup, serverConfig.TLSConfig)
		if err != nil {
			logrus.Fatal(err)
		}

		logrus.Debugf("Listener created for HTTP on %s (%s)", protoAddrParts[0], protoAddrParts[1])
		api.Accept(protoAddrParts[1], l...)
	}

	if err := migrateKey(); err != nil {
		logrus.Fatal(err)
	}
	cli.TrustKeyPath = commonFlags.TrustKey

	registryService := registry.NewService(cli.Config.ServiceOptions)
	containerdRemote, err := libcontainerd.New(cli.getLibcontainerdRoot(), cli.getPlatformRemoteOptions()...)
	if err != nil {
		logrus.Fatal(err)
	}

	d, err := daemon.NewDaemon(cli.Config, registryService, containerdRemote)
	if err != nil {
		if pfile != nil {
			if err := pfile.Remove(); err != nil {
				logrus.Error(err)
			}
		}
		logrus.Fatalf("Error starting daemon: %v", err)
	}

	logrus.Info("Daemon has completed initialization")

	logrus.WithFields(logrus.Fields{
		"version":     dockerversion.Version,
		"commit":      dockerversion.GitCommit,
		"graphdriver": d.GraphDriverName(),
	}).Info("Docker daemon")

	initRouter(api, d)

	reload := func(config *daemon.Config) {
		if err := d.Reload(config); err != nil {
			logrus.Errorf("Error reconfiguring the daemon: %v", err)
			return
		}
		if config.IsValueSet("debug") {
			debugEnabled := utils.IsDebugEnabled()
			switch {
			case debugEnabled && !config.Debug: // disable debug
				utils.DisableDebug()
				api.DisableProfiler()
			case config.Debug && !debugEnabled: // enable debug
				utils.EnableDebug()
				api.EnableProfiler()
			}

		}
	}

	setupConfigReloadTrap(*configFile, cli.flags, reload)

	// The serve API routine never exits unless an error occurs
	// We need to start it as a goroutine and wait on it so
	// daemon doesn't exit
	serveAPIWait := make(chan error)
	go api.Wait(serveAPIWait)

	signal.Trap(func() {
		api.Close()
		<-serveAPIWait
		shutdownDaemon(d, 15)
		if pfile != nil {
			if err := pfile.Remove(); err != nil {
				logrus.Error(err)
			}
		}
	})

	// after the daemon is done setting up we can notify systemd api
	notifySystem()

	// Daemon is fully initialized and handling API traffic
	// Wait for serve API to complete
	errAPI := <-serveAPIWait
	shutdownDaemon(d, 15)
	containerdRemote.Cleanup()
	if errAPI != nil {
		if pfile != nil {
			if err := pfile.Remove(); err != nil {
				logrus.Error(err)
			}
		}
		logrus.Fatalf("Shutting down due to ServeAPI error: %v", errAPI)
	}
	return nil
}

// shutdownDaemon just wraps daemon.Shutdown() to handle a timeout in case
// d.Shutdown() is waiting too long to kill container or worst it's
// blocked there
func shutdownDaemon(d *daemon.Daemon, timeout time.Duration) {
	ch := make(chan struct{})
	go func() {
		d.Shutdown()
		close(ch)
	}()
	select {
	case <-ch:
		logrus.Debug("Clean shutdown succeeded")
	case <-time.After(timeout * time.Second):
		logrus.Error("Force shutdown daemon")
	}
}

func loadDaemonCliConfig(config *daemon.Config, daemonFlags *flag.FlagSet, commonConfig *cli.CommonFlags, configFile string) (*daemon.Config, error) {
	config.Debug = commonConfig.Debug
	config.Hosts = commonConfig.Hosts
	config.LogLevel = commonConfig.LogLevel
	config.TLS = commonConfig.TLS
	config.TLSVerify = commonConfig.TLSVerify
	config.CommonTLSOptions = daemon.CommonTLSOptions{}

	if commonConfig.TLSOptions != nil {
		config.CommonTLSOptions.CAFile = commonConfig.TLSOptions.CAFile
		config.CommonTLSOptions.CertFile = commonConfig.TLSOptions.CertFile
		config.CommonTLSOptions.KeyFile = commonConfig.TLSOptions.KeyFile
	}

	if configFile != "" {
		c, err := daemon.MergeDaemonConfigurations(config, daemonFlags, configFile)
		if err != nil {
			if daemonFlags.IsSet(daemonConfigFileFlag) || !os.IsNotExist(err) {
				return nil, fmt.Errorf("unable to configure the Docker daemon with file %s: %v\n", configFile, err)
			}
		}
		// the merged configuration can be nil if the config file didn't exist.
		// leave the current configuration as it is if when that happens.
		if c != nil {
			config = c
		}
	}

	// Regardless of whether the user sets it to true or false, if they
	// specify TLSVerify at all then we need to turn on TLS
	if config.IsValueSet(tlsVerifyKey) {
		config.TLS = true
	}

	// ensure that the log level is the one set after merging configurations
	setDaemonLogLevel(config.LogLevel)

	return config, nil
}

func initRouter(s *apiserver.Server, d *daemon.Daemon) {
	decoder := runconfig.ContainerDecoder{}

	routers := []router.Router{
		container.NewRouter(d, decoder),
		image.NewRouter(d, decoder),
		systemrouter.NewRouter(d),
		volume.NewRouter(d),
		build.NewRouter(d),
	}
	if d.NetworkControllerEnabled() {
		routers = append(routers, network.NewRouter(d))
	}

	s.InitRouter(utils.IsDebugEnabled(), routers...)
}
