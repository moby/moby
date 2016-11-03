package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	dopts "github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/authorization"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/listeners"
	"github.com/docker/docker/pkg/pidfile"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/spf13/pflag"
)

const (
	flagDaemonConfigFile = "config-file"
)

// DaemonCli represents the daemon CLI.
type DaemonCli struct {
	*daemon.Config
	configFile *string
	flags      *pflag.FlagSet

	api             *apiserver.Server
	d               *daemon.Daemon
	authzMiddleware *authorization.Middleware // authzMiddleware enables to dynamically reload the authorization plugins
}

// NewDaemonCli returns a daemon CLI
func NewDaemonCli() *DaemonCli {
	return &DaemonCli{}
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

func (cli *DaemonCli) start(opts daemonOptions) (err error) {
	stopc := make(chan bool)
	defer close(stopc)

	// warn from uuid package when running the daemon
	uuid.Loggerf = logrus.Warnf

	opts.common.SetDefaultOptions(opts.flags)

	if opts.common.TrustKey == "" {
		opts.common.TrustKey = filepath.Join(
			getDaemonConfDir(),
			cliflags.DefaultTrustKeyFile)
	}
	if cli.Config, err = loadDaemonCliConfig(opts); err != nil {
		return err
	}
	cli.configFile = &opts.configFile
	cli.flags = opts.flags

	if cli.Config.Debug {
		utils.EnableDebug()
	}

	if cli.Config.Experimental {
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
		if cli.Config.Hosts[i], err = dopts.ParseHost(cli.Config.TLS, cli.Config.Hosts[i]); err != nil {
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
		logrus.Debugf("Listener created for HTTP on %s (%s)", proto, addr)
		api.Accept(addr, ls...)
	}

	if err := migrateKey(); err != nil {
		return err
	}
	// FIXME: why is this down here instead of with the other TrustKey logic above?
	cli.TrustKeyPath = opts.common.TrustKey

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

	if cli.Config.MetricsAddress != "" {
		if !d.HasExperimental() {
			return fmt.Errorf("metrics-addr is only supported when experimental is enabled")
		}
		if err := startMetricsServer(cli.Config.MetricsAddress); err != nil {
			return err
		}
	}

	name, _ := os.Hostname()

	c, err := cluster.New(cluster.Config{
		Root:                   cli.Config.Root,
		Name:                   name,
		Backend:                d,
		NetworkSubnetsProvider: d,
		DefaultAdvertiseAddr:   cli.Config.SwarmDefaultAdvertiseAddr,
		RuntimeRoot:            cli.getSwarmRunRoot(),
	})
	if err != nil {
		logrus.Fatalf("Error creating cluster component: %v", err)
	}

	// Restart all autostart containers which has a swarm endpoint
	// and is not yet running now that we have successfully
	// initialized the cluster.
	d.RestartSwarmContainers()

	logrus.Info("Daemon has completed initialization")

	logrus.WithFields(logrus.Fields{
		"version":     dockerversion.Version,
		"commit":      dockerversion.GitCommit,
		"graphdriver": d.GraphDriverName(),
	}).Info("Docker daemon")

	cli.d = d

	// initMiddlewares needs cli.d to be populated. Dont change this init order.
	if err := cli.initMiddlewares(api, serverConfig); err != nil {
		logrus.Fatalf("Error creating middlewares: %v", err)
	}
	d.SetCluster(c)
	initRouter(api, d, c)

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
	shutdownDaemon(d)
	containerdRemote.Cleanup()
	if errAPI != nil {
		return fmt.Errorf("Shutting down due to ServeAPI error: %v", errAPI)
	}

	return nil
}

func (cli *DaemonCli) reloadConfig() {
	reload := func(config *daemon.Config) {

		// Revalidate and reload the authorization plugins
		if err := validateAuthzPlugins(config.AuthorizationPlugins, cli.d.PluginStore); err != nil {
			logrus.Fatalf("Error validating authorization plugin: %v", err)
			return
		}
		cli.authzMiddleware.SetPlugins(config.AuthorizationPlugins)

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

	if err := daemon.ReloadConfiguration(*cli.configFile, cli.flags, reload); err != nil {
		logrus.Error(err)
	}
}

func (cli *DaemonCli) stop() {
	cli.api.Close()
}

// shutdownDaemon just wraps daemon.Shutdown() to handle a timeout in case
// d.Shutdown() is waiting too long to kill container or worst it's
// blocked there
func shutdownDaemon(d *daemon.Daemon) {
	shutdownTimeout := d.ShutdownTimeout()
	ch := make(chan struct{})
	go func() {
		d.Shutdown()
		close(ch)
	}()
	if shutdownTimeout < 0 {
		<-ch
		logrus.Debug("Clean shutdown succeeded")
		return
	}
	select {
	case <-ch:
		logrus.Debug("Clean shutdown succeeded")
	case <-time.After(time.Duration(shutdownTimeout) * time.Second):
		logrus.Error("Force shutdown daemon")
	}
}

func loadDaemonCliConfig(opts daemonOptions) (*daemon.Config, error) {
	config := opts.daemonConfig
	flags := opts.flags
	config.Debug = opts.common.Debug
	config.Hosts = opts.common.Hosts
	config.LogLevel = opts.common.LogLevel
	config.TLS = opts.common.TLS
	config.TLSVerify = opts.common.TLSVerify
	config.CommonTLSOptions = daemon.CommonTLSOptions{}

	if opts.common.TLSOptions != nil {
		config.CommonTLSOptions.CAFile = opts.common.TLSOptions.CAFile
		config.CommonTLSOptions.CertFile = opts.common.TLSOptions.CertFile
		config.CommonTLSOptions.KeyFile = opts.common.TLSOptions.KeyFile
	}

	if opts.configFile != "" {
		c, err := daemon.MergeDaemonConfigurations(config, flags, opts.configFile)
		if err != nil {
			if flags.Changed(flagDaemonConfigFile) || !os.IsNotExist(err) {
				return nil, fmt.Errorf("unable to configure the Docker daemon with file %s: %v\n", opts.configFile, err)
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

	// Labels of the docker engine used to allow multiple values associated with the same key.
	// This is deprecated in 1.13, and, be removed after 3 release cycles.
	// The following will check the conflict of labels, and report a warning for deprecation.
	//
	// TODO: After 3 release cycles (1.16) an error will be returned, and labels will be
	// sanitized to consolidate duplicate key-value pairs (config.Labels = newLabels):
	//
	// newLabels, err := daemon.GetConflictFreeLabels(config.Labels)
	// if err != nil {
	//	return nil, err
	// }
	// config.Labels = newLabels
	//
	if _, err := daemon.GetConflictFreeLabels(config.Labels); err != nil {
		logrus.Warnf("Engine labels with duplicate keys and conflicting values have been deprecated: %s", err)
	}

	// Regardless of whether the user sets it to true or false, if they
	// specify TLSVerify at all then we need to turn on TLS
	if config.IsValueSet(cliflags.FlagTLSVerify) {
		config.TLS = true
	}

	// ensure that the log level is the one set after merging configurations
	cliflags.SetLogLevel(config.LogLevel)

	return config, nil
}

func initRouter(s *apiserver.Server, d *daemon.Daemon, c *cluster.Cluster) {
	decoder := runconfig.ContainerDecoder{}

	routers := []router.Router{}

	// we need to add the checkpoint router before the container router or the DELETE gets masked
	routers = addExperimentalRouters(routers, d, decoder)

	routers = append(routers, []router.Router{
		container.NewRouter(d, decoder),
		image.NewRouter(d, decoder),
		systemrouter.NewRouter(d, c),
		volume.NewRouter(d),
		build.NewRouter(dockerfile.NewBuildManager(d)),
		swarmrouter.NewRouter(c),
	}...)

	if d.NetworkControllerEnabled() {
		routers = append(routers, network.NewRouter(d, c))
	}

	s.InitRouter(utils.IsDebugEnabled(), routers...)
}

func (cli *DaemonCli) initMiddlewares(s *apiserver.Server, cfg *apiserver.Config) error {
	v := cfg.Version

	exp := middleware.NewExperimentalMiddleware(cli.d.HasExperimental())
	s.UseMiddleware(exp)

	vm := middleware.NewVersionMiddleware(v, api.DefaultVersion, api.MinVersion)
	s.UseMiddleware(vm)

	if cfg.EnableCors {
		c := middleware.NewCORSMiddleware(cfg.CorsHeaders)
		s.UseMiddleware(c)
	}

	if err := validateAuthzPlugins(cli.Config.AuthorizationPlugins, cli.d.PluginStore); err != nil {
		return fmt.Errorf("Error validating authorization plugin: %v", err)
	}
	cli.authzMiddleware = authorization.NewMiddleware(cli.Config.AuthorizationPlugins, cli.d.PluginStore)
	s.UseMiddleware(cli.authzMiddleware)
	return nil
}

// validates that the plugins requested with the --authorization-plugin flag are valid AuthzDriver
// plugins present on the host and available to the daemon
func validateAuthzPlugins(requestedPlugins []string, pg plugingetter.PluginGetter) error {
	for _, reqPlugin := range requestedPlugins {
		if _, err := pg.Get(reqPlugin, authorization.AuthZApiImplements, plugingetter.LOOKUP); err != nil {
			return err
		}
	}
	return nil
}
