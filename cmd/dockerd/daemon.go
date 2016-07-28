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
	"github.com/docker/docker/api"
	apiserver "github.com/docker/docker/api/server"
	"github.com/docker/docker/api/server/middleware"
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/build"
	"github.com/docker/docker/api/server/router/container"
	"github.com/docker/docker/api/server/router/image"
	"github.com/docker/docker/api/server/router/network"
	swarmrouter "github.com/docker/docker/api/server/router/swarm"
	systemrouter "github.com/docker/docker/api/server/router/system"
	"github.com/docker/docker/api/server/router/volume"
	"github.com/docker/docker/builder/dockerfile"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/cluster"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/authorization"
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
	daemonConfigFileFlag = "-config-file"
)

// DaemonCli represents the daemon CLI.
type DaemonCli struct {
	*daemon.Config
	commonFlags *cliflags.CommonFlags
	configFile  *string

	api *apiserver.Server
	d   *daemon.Daemon
}

func presentInHelp(usage string) string { return usage }
func absentFromHelp(string) string      { return "" }

// NewDaemonCli returns a pre-configured daemon CLI
func NewDaemonCli() *DaemonCli {
	// TODO(tiborvass): remove InstallFlags?
	daemonConfig := new(daemon.Config)
	daemonConfig.LogConfig.Config = make(map[string]string)
	daemonConfig.ClusterOpts = make(map[string]string)

	if runtime.GOOS != "linux" {
		daemonConfig.V2Only = true
	}

	daemonConfig.InstallFlags(flag.CommandLine, presentInHelp)
	configFile := flag.CommandLine.String([]string{daemonConfigFileFlag}, defaultDaemonConfigFile, "Daemon configuration file")
	flag.CommandLine.Require(flag.Exact, 0)

	return &DaemonCli{
		Config:      daemonConfig,
		commonFlags: cliflags.InitCommonFlags(),
		configFile:  configFile,
	}
}

func migrateKey() (err error) {
	// Migrate trust key if exists at ~/.docker/key.json and owned by current user
	oldPath := filepath.Join(cliconfig.ConfigDir(), cliflags.DefaultTrustKeyFile)
	newPath := filepath.Join(getDaemonConfDir(), cliflags.DefaultTrustKeyFile)
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

func (cli *DaemonCli) start() (err error) {
	stopc := make(chan bool)
	defer close(stopc)

	// warn from uuid package when running the daemon
	uuid.Loggerf = logrus.Warnf

	flags := flag.CommandLine
	cli.commonFlags.PostParse()

	if cli.commonFlags.TrustKey == "" {
		cli.commonFlags.TrustKey = filepath.Join(getDaemonConfDir(), cliflags.DefaultTrustKeyFile)
	}
	cliConfig, err := loadDaemonCliConfig(cli.Config, flags, cli.commonFlags, *cli.configFile)
	if err != nil {
		return err
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
		return fmt.Errorf("Failed to set umask: %v", err)
	}

	if len(cli.LogConfig.Config) > 0 {
		if err := logger.ValidateLogOpts(cli.LogConfig.Type, cli.LogConfig.Config); err != nil {
			return fmt.Errorf("Failed to set log opts: %v", err)
		}
	}

	if cli.Pidfile != "" {
		pf, err := pidfile.New(cli.Pidfile)
		if err != nil {
			return fmt.Errorf("Error starting daemon: %v", err)
		}
		defer func() {
			if err := pf.Remove(); err != nil {
				logrus.Error(err)
			}
		}()
	}

	serverConfig := &apiserver.Config{
		Logging:     true,
		SocketGroup: cli.Config.SocketGroup,
		Version:     dockerversion.Version,
		EnableCors:  cli.Config.EnableCors,
		CorsHeaders: cli.Config.CorsHeaders,
	}

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
			return err
		}
		serverConfig.TLSConfig = tlsConfig
	}

	if len(cli.Config.Hosts) == 0 {
		cli.Config.Hosts = make([]string, 1)
	}

	api := apiserver.New(serverConfig)
	cli.api = api

	for i := 0; i < len(cli.Config.Hosts); i++ {
		var err error
		if cli.Config.Hosts[i], err = opts.ParseHost(cli.Config.TLS, cli.Config.Hosts[i]); err != nil {
			return fmt.Errorf("error parsing -H %s : %v", cli.Config.Hosts[i], err)
		}

		protoAddr := cli.Config.Hosts[i]
		protoAddrParts := strings.SplitN(protoAddr, "://", 2)
		if len(protoAddrParts) != 2 {
			return fmt.Errorf("bad format %s, expected PROTO://ADDR", protoAddr)
		}

		proto := protoAddrParts[0]
		addr := protoAddrParts[1]

		// It's a bad idea to bind to TCP without tlsverify.
		if proto == "tcp" && (serverConfig.TLSConfig == nil || serverConfig.TLSConfig.ClientAuth != tls.RequireAndVerifyClientCert) {
			logrus.Warn("[!] DON'T BIND ON ANY IP ADDRESS WITHOUT setting -tlsverify IF YOU DON'T KNOW WHAT YOU'RE DOING [!]")
		}
		ls, err := listeners.Init(proto, addr, serverConfig.SocketGroup, serverConfig.TLSConfig)
		if err != nil {
			return err
		}
		ls = wrapListeners(proto, ls)
		// If we're binding to a TCP port, make sure that a container doesn't try to use it.
		if proto == "tcp" {
			if err := allocateDaemonPort(addr); err != nil {
				return err
			}
		}
		logrus.Debugf("Listener created for HTTP on %s (%s)", protoAddrParts[0], protoAddrParts[1])
		api.Accept(protoAddrParts[1], ls...)
	}

	if err := migrateKey(); err != nil {
		return err
	}
	cli.TrustKeyPath = cli.commonFlags.TrustKey

	registryService := registry.NewService(cli.Config.ServiceOptions)
	containerdRemote, err := libcontainerd.New(cli.getLibcontainerdRoot(), cli.getPlatformRemoteOptions()...)
	if err != nil {
		return err
	}
	signal.Trap(func() {
		cli.stop()
		<-stopc // wait for daemonCli.start() to return
	})

	d, err := daemon.NewDaemon(cli.Config, registryService, containerdRemote)
	if err != nil {
		return fmt.Errorf("Error starting daemon: %v", err)
	}

	name, _ := os.Hostname()

	c, err := cluster.New(cluster.Config{
		Root:                   cli.Config.Root,
		Name:                   name,
		Backend:                d,
		NetworkSubnetsProvider: d,
		DefaultAdvertiseAddr:   cli.Config.SwarmDefaultAdvertiseAddr,
	})
	if err != nil {
		logrus.Fatalf("Error creating cluster component: %v", err)
	}

	logrus.Info("Daemon has completed initialization")

	logrus.WithFields(logrus.Fields{
		"version":     dockerversion.Version,
		"commit":      dockerversion.GitCommit,
		"graphdriver": d.GraphDriverName(),
	}).Info("Docker daemon")

	cli.initMiddlewares(api, serverConfig)
	initRouter(api, d, c)

	cli.d = d
	cli.setupConfigReloadTrap()

	// The serve API routine never exits unless an error occurs
	// We need to start it as a goroutine and wait on it so
	// daemon doesn't exit
	serveAPIWait := make(chan error)
	go api.Wait(serveAPIWait)

	// after the daemon is done setting up we can notify systemd api
	notifySystem()

	// Daemon is fully initialized and handling API traffic
	// Wait for serve API to complete
	errAPI := <-serveAPIWait
	c.Cleanup()
	shutdownDaemon(d, 15)
	containerdRemote.Cleanup()
	if errAPI != nil {
		return fmt.Errorf("Shutting down due to ServeAPI error: %v", errAPI)
	}

	return nil
}

func (cli *DaemonCli) reloadConfig() {
	reload := func(config *daemon.Config) {
		if err := cli.d.Reload(config); err != nil {
			logrus.Errorf("Error reconfiguring the daemon: %v", err)
			return
		}
		if config.IsValueSet("debug") {
			debugEnabled := utils.IsDebugEnabled()
			switch {
			case debugEnabled && !config.Debug: // disable debug
				utils.DisableDebug()
				cli.api.DisableProfiler()
			case config.Debug && !debugEnabled: // enable debug
				utils.EnableDebug()
				cli.api.EnableProfiler()
			}

		}
	}

	if err := daemon.ReloadConfiguration(*cli.configFile, flag.CommandLine, reload); err != nil {
		logrus.Error(err)
	}
}

func (cli *DaemonCli) stop() {
	cli.api.Close()
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

func loadDaemonCliConfig(config *daemon.Config, flags *flag.FlagSet, commonConfig *cliflags.CommonFlags, configFile string) (*daemon.Config, error) {
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
		c, err := daemon.MergeDaemonConfigurations(config, flags, configFile)
		if err != nil {
			if flags.IsSet(daemonConfigFileFlag) || !os.IsNotExist(err) {
				return nil, fmt.Errorf("unable to configure the Docker daemon with file %s: %v\n", configFile, err)
			}
		}
		// the merged configuration can be nil if the config file didn't exist.
		// leave the current configuration as it is if when that happens.
		if c != nil {
			config = c
		}
	}

	if err := daemon.ValidateConfiguration(config); err != nil {
		return nil, err
	}

	// Regardless of whether the user sets it to true or false, if they
	// specify TLSVerify at all then we need to turn on TLS
	if config.IsValueSet(cliflags.TLSVerifyKey) {
		config.TLS = true
	}

	// ensure that the log level is the one set after merging configurations
	cliflags.SetDaemonLogLevel(config.LogLevel)

	return config, nil
}

func initRouter(s *apiserver.Server, d *daemon.Daemon, c *cluster.Cluster) {
	decoder := runconfig.ContainerDecoder{}

	routers := []router.Router{
		container.NewRouter(d, decoder),
		image.NewRouter(d, decoder),
		systemrouter.NewRouter(d, c),
		volume.NewRouter(d),
		build.NewRouter(dockerfile.NewBuildManager(d)),
		swarmrouter.NewRouter(c),
	}
	if d.NetworkControllerEnabled() {
		routers = append(routers, network.NewRouter(d, c))
	}
	routers = addExperimentalRouters(routers)

	s.InitRouter(utils.IsDebugEnabled(), routers...)
}

func (cli *DaemonCli) initMiddlewares(s *apiserver.Server, cfg *apiserver.Config) {
	v := cfg.Version

	vm := middleware.NewVersionMiddleware(v, api.DefaultVersion, api.MinVersion)
	s.UseMiddleware(vm)

	if cfg.EnableCors {
		c := middleware.NewCORSMiddleware(cfg.CorsHeaders)
		s.UseMiddleware(c)
	}

	u := middleware.NewUserAgentMiddleware(v)
	s.UseMiddleware(u)

	if len(cli.Config.AuthorizationPlugins) > 0 {
		authZPlugins := authorization.NewPlugins(cli.Config.AuthorizationPlugins)
		handleAuthorization := authorization.NewMiddleware(authZPlugins)
		s.UseMiddleware(handleAuthorization)
	}
}
