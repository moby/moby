package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	containerddefaults "github.com/containerd/containerd/defaults"
	"github.com/docker/docker/api"
	apiserver "github.com/docker/docker/api/server"
	buildbackend "github.com/docker/docker/api/server/backend/build"
	"github.com/docker/docker/api/server/middleware"
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/build"
	checkpointrouter "github.com/docker/docker/api/server/router/checkpoint"
	"github.com/docker/docker/api/server/router/container"
	distributionrouter "github.com/docker/docker/api/server/router/distribution"
	grpcrouter "github.com/docker/docker/api/server/router/grpc"
	"github.com/docker/docker/api/server/router/image"
	"github.com/docker/docker/api/server/router/network"
	pluginrouter "github.com/docker/docker/api/server/router/plugin"
	sessionrouter "github.com/docker/docker/api/server/router/session"
	swarmrouter "github.com/docker/docker/api/server/router/swarm"
	systemrouter "github.com/docker/docker/api/server/router/system"
	"github.com/docker/docker/api/server/router/volume"
	buildkit "github.com/docker/docker/builder/builder-next"
	"github.com/docker/docker/builder/dockerfile"
	"github.com/docker/docker/cli/debug"
	"github.com/docker/docker/cmd/dockerd/trap"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/cluster"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/listeners"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/libcontainerd/supervisor"
	dopts "github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/authorization"
	"github.com/docker/docker/pkg/homedir"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/pidfile"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/plugin"
	"github.com/docker/docker/rootless"
	"github.com/docker/docker/runconfig"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/moby/buildkit/session"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

// DaemonCli represents the daemon CLI.
type DaemonCli struct {
	*config.Config
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

func (cli *DaemonCli) start(opts *daemonOptions) (err error) {
	if cli.Config, err = loadDaemonCliConfig(opts); err != nil {
		return err
	}
	if err := checkDeprecatedOptions(cli.Config); err != nil {
		return err
	}

	serverConfig, err := newAPIServerConfig(cli.Config)
	if err != nil {
		return err
	}

	if opts.Validate {
		// If config wasn't OK we wouldn't have made it this far.
		_, _ = fmt.Fprintln(os.Stderr, "configuration OK")
		return nil
	}

	configureProxyEnv(cli.Config)
	configureDaemonLogs(cli.Config)

	logrus.Info("Starting up")

	cli.configFile = &opts.configFile
	cli.flags = opts.flags

	if cli.Config.Debug {
		debug.Enable()
	}

	if cli.Config.Experimental {
		logrus.Warn("Running experimental build")
	}

	if cli.Config.IsRootless() {
		logrus.Warn("Running in rootless mode. This mode has feature limitations.")
	}
	if rootless.RunningWithRootlessKit() {
		logrus.Info("Running with RootlessKit integration")
		if !cli.Config.IsRootless() {
			return fmt.Errorf("rootless mode needs to be enabled for running with RootlessKit")
		}
	}

	// return human-friendly error before creating files
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		return fmt.Errorf("dockerd needs to be started with root privileges. To run dockerd in rootless mode as an unprivileged user, see https://docs.docker.com/go/rootless/")
	}

	if err := setDefaultUmask(); err != nil {
		return err
	}

	// Create the daemon root before we create ANY other files (PID, or migrate keys)
	// to ensure the appropriate ACL is set (particularly relevant on Windows)
	if err := daemon.CreateDaemonRoot(cli.Config); err != nil {
		return err
	}

	if err := system.MkdirAll(cli.Config.ExecRoot, 0700); err != nil {
		return err
	}

	potentiallyUnderRuntimeDir := []string{cli.Config.ExecRoot}

	if cli.Pidfile != "" {
		pf, err := pidfile.New(cli.Pidfile)
		if err != nil {
			return errors.Wrap(err, "failed to start daemon")
		}
		potentiallyUnderRuntimeDir = append(potentiallyUnderRuntimeDir, cli.Pidfile)
		defer func() {
			if err := pf.Remove(); err != nil {
				logrus.Error(err)
			}
		}()
	}

	if cli.Config.IsRootless() {
		// Set sticky bit if XDG_RUNTIME_DIR is set && the file is actually under XDG_RUNTIME_DIR
		if _, err := homedir.StickRuntimeDirContents(potentiallyUnderRuntimeDir); err != nil {
			// StickRuntimeDirContents returns nil error if XDG_RUNTIME_DIR is just unset
			logrus.WithError(err).Warn("cannot set sticky bit on files under XDG_RUNTIME_DIR")
		}
	}

	cli.api = apiserver.New(serverConfig)

	hosts, err := loadListeners(cli, serverConfig)
	if err != nil {
		return errors.Wrap(err, "failed to load listeners")
	}

	ctx, cancel := context.WithCancel(context.Background())
	waitForContainerDShutdown, err := cli.initContainerD(ctx)
	if waitForContainerDShutdown != nil {
		defer waitForContainerDShutdown(10 * time.Second)
	}
	if err != nil {
		cancel()
		return err
	}
	defer cancel()

	stopc := make(chan bool)
	defer close(stopc)

	trap.Trap(func() {
		cli.stop()
		<-stopc // wait for daemonCli.start() to return
	}, logrus.StandardLogger())

	// Notify that the API is active, but before daemon is set up.
	preNotifyReady()

	pluginStore := plugin.NewStore()

	if err := cli.initMiddlewares(cli.api, serverConfig, pluginStore); err != nil {
		logrus.Fatalf("Error creating middlewares: %v", err)
	}

	d, err := daemon.NewDaemon(ctx, cli.Config, pluginStore)
	if err != nil {
		return errors.Wrap(err, "failed to start daemon")
	}

	d.StoreHosts(hosts)

	// validate after NewDaemon has restored enabled plugins. Don't change order.
	if err := validateAuthzPlugins(cli.Config.AuthorizationPlugins, pluginStore); err != nil {
		return errors.Wrap(err, "failed to validate authorization plugin")
	}

	cli.d = d

	if err := startMetricsServer(cli.Config.MetricsAddress); err != nil {
		return errors.Wrap(err, "failed to start metrics server")
	}

	c, err := createAndStartCluster(cli, d)
	if err != nil {
		logrus.Fatalf("Error starting cluster component: %v", err)
	}

	// Restart all autostart containers which has a swarm endpoint
	// and is not yet running now that we have successfully
	// initialized the cluster.
	d.RestartSwarmContainers()

	logrus.Info("Daemon has completed initialization")

	routerOptions, err := newRouterOptions(cli.Config, d)
	if err != nil {
		return err
	}
	routerOptions.api = cli.api
	routerOptions.cluster = c

	initRouter(d, routerOptions)

	go d.ProcessClusterNotifications(ctx, c.GetWatchStream())

	cli.setupConfigReloadTrap()

	// The serve API routine never exits unless an error occurs
	// We need to start it as a goroutine and wait on it so
	// daemon doesn't exit
	serveAPIWait := make(chan error)
	go cli.api.Wait(serveAPIWait)

	// after the daemon is done setting up we can notify systemd api
	notifyReady()

	// Daemon is fully initialized and handling API traffic
	// Wait for serve API to complete
	errAPI := <-serveAPIWait
	c.Cleanup()

	// notify systemd that we're shutting down
	notifyStopping()
	shutdownDaemon(d)

	// Stop notification processing and any background processes
	cancel()

	if errAPI != nil {
		return errors.Wrap(errAPI, "shutting down due to ServeAPI error")
	}

	logrus.Info("Daemon shutdown complete")
	return nil
}

type routerOptions struct {
	sessionManager *session.Manager
	buildBackend   *buildbackend.Backend
	features       *map[string]bool
	buildkit       *buildkit.Builder
	daemon         *daemon.Daemon
	api            *apiserver.Server
	cluster        *cluster.Cluster
}

func newRouterOptions(config *config.Config, d *daemon.Daemon) (routerOptions, error) {
	opts := routerOptions{}
	sm, err := session.NewManager()
	if err != nil {
		return opts, errors.Wrap(err, "failed to create sessionmanager")
	}

	manager, err := dockerfile.NewBuildManager(d.BuilderBackend(), d.IdentityMapping())
	if err != nil {
		return opts, err
	}
	cgroupParent := newCgroupParent(config)
	ro := routerOptions{
		sessionManager: sm,
		features:       d.Features(),
		daemon:         d,
	}
	if !d.UsesSnapshotter() {
		bk, err := buildkit.New(buildkit.Opt{
			SessionManager:      sm,
			Root:                filepath.Join(config.Root, "buildkit"),
			Dist:                d.DistributionServices(),
			NetworkController:   d.NetworkController(),
			DefaultCgroupParent: cgroupParent,
			RegistryHosts:       d.RegistryHosts(),
			BuilderConfig:       config.Builder,
			Rootless:            d.Rootless(),
			IdentityMapping:     d.IdentityMapping(),
			DNSConfig:           config.DNSConfig,
			ApparmorProfile:     daemon.DefaultApparmorProfile(),
		})
		if err != nil {
			return opts, err
		}

		bb, err := buildbackend.NewBackend(d.ImageService(), manager, bk, d.EventsService)
		if err != nil {
			return opts, errors.Wrap(err, "failed to create buildmanager")
		}

		ro.buildBackend = bb
		ro.buildkit = bk
	}

	return ro, nil
}

func (cli *DaemonCli) reloadConfig() {
	reload := func(c *config.Config) {

		// Revalidate and reload the authorization plugins
		if err := validateAuthzPlugins(c.AuthorizationPlugins, cli.d.PluginStore); err != nil {
			logrus.Fatalf("Error validating authorization plugin: %v", err)
			return
		}
		cli.authzMiddleware.SetPlugins(c.AuthorizationPlugins)

		if err := cli.d.Reload(c); err != nil {
			logrus.Errorf("Error reconfiguring the daemon: %v", err)
			return
		}

		if c.IsValueSet("debug") {
			debugEnabled := debug.IsEnabled()
			switch {
			case debugEnabled && !c.Debug: // disable debug
				debug.Disable()
			case c.Debug && !debugEnabled: // enable debug
				debug.Enable()
			}
		}
	}

	if err := config.Reload(*cli.configFile, cli.flags, reload); err != nil {
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

	timeout := time.NewTimer(time.Duration(shutdownTimeout) * time.Second)
	defer timeout.Stop()

	select {
	case <-ch:
		logrus.Debug("Clean shutdown succeeded")
	case <-timeout.C:
		logrus.Error("Force shutdown daemon")
	}
}

func loadDaemonCliConfig(opts *daemonOptions) (*config.Config, error) {
	if !opts.flags.Parsed() {
		return nil, errors.New(`cannot load CLI config before flags are parsed`)
	}
	opts.setDefaultOptions()

	conf := opts.daemonConfig
	flags := opts.flags
	conf.Debug = opts.Debug
	conf.Hosts = opts.Hosts
	conf.LogLevel = opts.LogLevel

	if flags.Changed("graph") && flags.Changed("data-root") {
		return nil, errors.New(`cannot specify both "--graph" and "--data-root" option`)
	}
	if flags.Changed(FlagTLS) {
		conf.TLS = &opts.TLS
	}
	if flags.Changed(FlagTLSVerify) {
		conf.TLSVerify = &opts.TLSVerify
		v := true
		conf.TLS = &v
	}

	if opts.TLSOptions != nil {
		conf.CommonTLSOptions = config.CommonTLSOptions{
			CAFile:   opts.TLSOptions.CAFile,
			CertFile: opts.TLSOptions.CertFile,
			KeyFile:  opts.TLSOptions.KeyFile,
		}
	} else {
		conf.CommonTLSOptions = config.CommonTLSOptions{}
	}

	if conf.TrustKeyPath == "" {
		daemonConfDir, err := getDaemonConfDir(conf.Root)
		if err != nil {
			return nil, err
		}
		conf.TrustKeyPath = filepath.Join(daemonConfDir, defaultTrustKeyFile)
	}

	if opts.configFile != "" {
		c, err := config.MergeDaemonConfigurations(conf, flags, opts.configFile)
		if err != nil {
			if flags.Changed("config-file") || !os.IsNotExist(err) {
				return nil, errors.Wrapf(err, "unable to configure the Docker daemon with file %s", opts.configFile)
			}
		}

		// the merged configuration can be nil if the config file didn't exist.
		// leave the current configuration as it is if when that happens.
		if c != nil {
			conf = c
		}
	}

	if err := normalizeHosts(conf); err != nil {
		return nil, err
	}

	if err := config.Validate(conf); err != nil {
		return nil, err
	}

	if flags.Changed("graph") {
		logrus.Warnf(`The "-g / --graph" flag is deprecated. Please use "--data-root" instead`)
	}

	// Check if duplicate label-keys with different values are found
	newLabels, err := config.GetConflictFreeLabels(conf.Labels)
	if err != nil {
		return nil, err
	}
	conf.Labels = newLabels

	// Regardless of whether the user sets it to true or false, if they
	// specify TLSVerify at all then we need to turn on TLS
	if conf.IsValueSet(FlagTLSVerify) {
		v := true
		conf.TLS = &v
	}

	if conf.TLSVerify == nil && conf.TLS != nil {
		conf.TLSVerify = conf.TLS
	}

	err = validateCPURealtimeOptions(conf)
	if err != nil {
		return nil, err
	}

	return conf, nil
}

// normalizeHosts normalizes the configured config.Hosts and remove duplicates.
// It returns an error if it fails to parse a host.
func normalizeHosts(config *config.Config) error {
	if len(config.Hosts) == 0 {
		// if no hosts are configured, create a single entry slice, so that the
		// default is used.
		//
		// TODO(thaJeztah) implement a cleaner way for this; this depends on a
		//                 side-effect of how we parse empty/partial hosts.
		config.Hosts = make([]string, 1)
	}
	hosts := make([]string, 0, len(config.Hosts))
	seen := make(map[string]struct{}, len(config.Hosts))

	useTLS := DefaultTLSValue
	if config.TLS != nil {
		useTLS = *config.TLS
	}

	for _, h := range config.Hosts {
		host, err := dopts.ParseHost(useTLS, honorXDG, h)
		if err != nil {
			return err
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	config.Hosts = hosts
	return nil
}

func checkDeprecatedOptions(config *config.Config) error {
	// Overlay networks with external k/v stores have been deprecated
	if config.ClusterAdvertise != "" || len(config.ClusterOpts) > 0 || config.ClusterStore != "" {
		return errors.New("Host-discovery and overlay networks with external k/v stores are deprecated. The 'cluster-advertise', 'cluster-store', and 'cluster-store-opt' options have been removed")
	}
	return nil
}

func initRouter(d *daemon.Daemon, opts routerOptions) {
	decoder := runconfig.ContainerDecoder{
		GetSysInfo: func() *sysinfo.SysInfo {
			return opts.daemon.RawSysInfo()
		},
	}

	routers := []router.Router{
		// we need to add the checkpoint router before the container router or the DELETE gets masked
		checkpointrouter.NewRouter(opts.daemon, decoder),
		container.NewRouter(opts.daemon, decoder, opts.daemon.RawSysInfo().CgroupUnified),
		image.NewRouter(
			opts.daemon.ImageService(),
			opts.daemon.ReferenceStore,
			opts.daemon.ImageService().DistributionServices().ImageStore,
			opts.daemon.ImageService().DistributionServices().LayerStore,
		),
		systemrouter.NewRouter(opts.daemon, opts.cluster, opts.buildkit, opts.features),
		volume.NewRouter(opts.daemon.VolumesService(), opts.cluster),
		build.NewRouter(opts.buildBackend, opts.daemon, opts.features),
		sessionrouter.NewRouter(opts.sessionManager),
		swarmrouter.NewRouter(opts.cluster),
		pluginrouter.NewRouter(opts.daemon.PluginManager()),
		distributionrouter.NewRouter(opts.daemon.ImageService()),
	}

	grpcBackends := []grpcrouter.Backend{}
	backends := []interface{}{opts.daemon}
	if !d.UsesSnapshotter() {
		backends = append(backends, opts.buildBackend)
	}
	for _, b := range backends {
		if b, ok := b.(grpcrouter.Backend); ok {
			grpcBackends = append(grpcBackends, b)
		}
	}
	if len(grpcBackends) > 0 {
		routers = append(routers, grpcrouter.NewRouter(grpcBackends...))
	}

	if opts.daemon.NetworkControllerEnabled() {
		routers = append(routers, network.NewRouter(opts.daemon, opts.cluster))
	}

	if opts.daemon.HasExperimental() {
		for _, r := range routers {
			for _, route := range r.Routes() {
				if experimental, ok := route.(router.ExperimentalRoute); ok {
					experimental.Enable()
				}
			}
		}
	}

	opts.api.InitRouter(routers...)
}

// TODO: remove this from cli and return the authzMiddleware
func (cli *DaemonCli) initMiddlewares(s *apiserver.Server, cfg *apiserver.Config, pluginStore plugingetter.PluginGetter) error {
	v := cfg.Version

	exp := middleware.NewExperimentalMiddleware(cli.Config.Experimental)
	s.UseMiddleware(exp)

	vm := middleware.NewVersionMiddleware(v, api.DefaultVersion, api.MinVersion)
	s.UseMiddleware(vm)

	if cfg.CorsHeaders != "" {
		c := middleware.NewCORSMiddleware(cfg.CorsHeaders)
		s.UseMiddleware(c)
	}

	cli.authzMiddleware = authorization.NewMiddleware(cli.Config.AuthorizationPlugins, pluginStore)
	cli.Config.AuthzMiddleware = cli.authzMiddleware
	s.UseMiddleware(cli.authzMiddleware)
	return nil
}

func (cli *DaemonCli) getContainerdDaemonOpts() ([]supervisor.DaemonOpt, error) {
	opts, err := cli.getPlatformContainerdDaemonOpts()
	if err != nil {
		return nil, err
	}

	if cli.Config.Debug {
		opts = append(opts, supervisor.WithLogLevel("debug"))
	} else if cli.Config.LogLevel != "" {
		opts = append(opts, supervisor.WithLogLevel(cli.Config.LogLevel))
	}

	if !cli.Config.CriContainerd {
		opts = append(opts, supervisor.WithPlugin("cri", nil))
	}

	return opts, nil
}

func newAPIServerConfig(config *config.Config) (*apiserver.Config, error) {
	var tlsConfig *tls.Config
	if config.TLS != nil && *config.TLS {
		var (
			clientAuth tls.ClientAuthType
			err        error
		)
		if config.TLSVerify == nil || *config.TLSVerify {
			// server requires and verifies client's certificate
			clientAuth = tls.RequireAndVerifyClientCert
		}
		tlsConfig, err = tlsconfig.Server(tlsconfig.Options{
			CAFile:             config.CommonTLSOptions.CAFile,
			CertFile:           config.CommonTLSOptions.CertFile,
			KeyFile:            config.CommonTLSOptions.KeyFile,
			ExclusiveRootPools: true,
			ClientAuth:         clientAuth,
		})
		if err != nil {
			return nil, errors.Wrap(err, "invalid TLS configuration")
		}
	}

	return &apiserver.Config{
		SocketGroup: config.SocketGroup,
		Version:     dockerversion.Version,
		CorsHeaders: config.CorsHeaders,
		TLSConfig:   tlsConfig,
		Hosts:       config.Hosts,
	}, nil
}

// checkTLSAuthOK checks basically for an explicitly disabled TLS/TLSVerify
// Going forward we do not want to support a scenario where dockerd listens
//   on TCP without either TLS client auth (or an explicit opt-in to disable it)
func checkTLSAuthOK(c *config.Config) bool {
	if c.TLS == nil {
		// Either TLS is enabled by default, in which case TLS verification should be enabled by default, or explicitly disabled
		// Or TLS is disabled by default... in any of these cases, we can just take the default value as to how to proceed
		return DefaultTLSValue
	}

	if !*c.TLS {
		// TLS is explicitly disabled, which is supported
		return true
	}

	if c.TLSVerify == nil {
		// this actually shouldn't happen since we set TLSVerify on the config object anyway
		// But in case it does get here, be cautious and assume this is not supported.
		return false
	}

	// Either TLSVerify is explicitly enabled or disabled, both cases are supported
	return true
}

func loadListeners(cli *DaemonCli, serverConfig *apiserver.Config) ([]string, error) {
	if len(serverConfig.Hosts) == 0 {
		return nil, errors.New("no hosts configured")
	}
	var hosts []string

	for i := 0; i < len(serverConfig.Hosts); i++ {
		protoAddr := serverConfig.Hosts[i]
		protoAddrParts := strings.SplitN(serverConfig.Hosts[i], "://", 2)
		if len(protoAddrParts) != 2 {
			return nil, fmt.Errorf("bad format %s, expected PROTO://ADDR", protoAddr)
		}

		proto, addr := protoAddrParts[0], protoAddrParts[1]

		// It's a bad idea to bind to TCP without tlsverify.
		authEnabled := serverConfig.TLSConfig != nil && serverConfig.TLSConfig.ClientAuth == tls.RequireAndVerifyClientCert
		if proto == "tcp" && !authEnabled {
			logrus.WithField("host", protoAddr).Warn("Binding to IP address without --tlsverify is insecure and gives root access on this machine to everyone who has access to your network.")
			logrus.WithField("host", protoAddr).Warn("Binding to an IP address, even on localhost, can also give access to scripts run in a browser. Be safe out there!")
			time.Sleep(time.Second)

			// If TLSVerify is explicitly set to false we'll take that as "Please let me shoot myself in the foot"
			// We do not want to continue to support a default mode where tls verification is disabled, so we do some extra warnings here and eventually remove support
			if !checkTLSAuthOK(cli.Config) {
				ipAddr, _, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, errors.Wrap(err, "error parsing tcp address")
				}

				// shortcut all this extra stuff for literal "localhost"
				// -H supports specifying hostnames, since we want to bypass this on loopback interfaces we'll look it up here.
				if ipAddr != "localhost" {
					ip := net.ParseIP(ipAddr)
					if ip == nil {
						ipA, err := net.ResolveIPAddr("ip", ipAddr)
						if err != nil {
							logrus.WithError(err).WithField("host", ipAddr).Error("Error looking up specified host address")
						}
						if ipA != nil {
							ip = ipA.IP
						}
					}
					if ip == nil || !ip.IsLoopback() {
						logrus.WithField("host", protoAddr).Warn("Binding to an IP address without --tlsverify is deprecated. Startup is intentionally being slowed down to show this message")
						logrus.WithField("host", protoAddr).Warn("Please consider generating tls certificates with client validation to prevent exposing unauthenticated root access to your network")
						logrus.WithField("host", protoAddr).Warnf("You can override this by explicitly specifying '--%s=false' or '--%s=false'", FlagTLS, FlagTLSVerify)
						logrus.WithField("host", protoAddr).Warnf("Support for listening on TCP without authentication or explicit intent to run without authentication will be removed in the next release")

						time.Sleep(15 * time.Second)
					}
				}
			}
		}
		// If we're binding to a TCP port, make sure that a container doesn't try to use it.
		if proto == "tcp" {
			if err := allocateDaemonPort(addr); err != nil {
				return nil, err
			}
		}
		ls, err := listeners.Init(proto, addr, serverConfig.SocketGroup, serverConfig.TLSConfig)
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Listener created for HTTP on %s (%s)", proto, addr)
		hosts = append(hosts, protoAddrParts[1])
		cli.api.Accept(addr, ls...)
	}

	return hosts, nil
}

func createAndStartCluster(cli *DaemonCli, d *daemon.Daemon) (*cluster.Cluster, error) {
	name, _ := os.Hostname()

	// Use a buffered channel to pass changes from store watch API to daemon
	// A buffer allows store watch API and daemon processing to not wait for each other
	watchStream := make(chan *swarmapi.WatchMessage, 32)

	c, err := cluster.New(cluster.Config{
		Root:                   cli.Config.Root,
		Name:                   name,
		Backend:                d,
		VolumeBackend:          d.VolumesService(),
		ImageBackend:           d.ImageService(),
		PluginBackend:          d.PluginManager(),
		NetworkSubnetsProvider: d,
		DefaultAdvertiseAddr:   cli.Config.SwarmDefaultAdvertiseAddr,
		RaftHeartbeatTick:      cli.Config.SwarmRaftHeartbeatTick,
		RaftElectionTick:       cli.Config.SwarmRaftElectionTick,
		RuntimeRoot:            cli.getSwarmRunRoot(),
		WatchStream:            watchStream,
	})
	if err != nil {
		return nil, err
	}
	d.SetCluster(c)
	err = c.Start()

	return c, err
}

// validates that the plugins requested with the --authorization-plugin flag are valid AuthzDriver
// plugins present on the host and available to the daemon
func validateAuthzPlugins(requestedPlugins []string, pg plugingetter.PluginGetter) error {
	for _, reqPlugin := range requestedPlugins {
		if _, err := pg.Get(reqPlugin, authorization.AuthZApiImplements, plugingetter.Lookup); err != nil {
			return err
		}
	}
	return nil
}

func systemContainerdRunning(honorXDG bool) (string, bool, error) {
	addr := containerddefaults.DefaultAddress
	if honorXDG {
		runtimeDir, err := homedir.GetRuntimeDir()
		if err != nil {
			return "", false, err
		}
		addr = filepath.Join(runtimeDir, "containerd", "containerd.sock")
	}
	_, err := os.Lstat(addr)
	return addr, err == nil, nil
}

// configureDaemonLogs sets the logrus logging level and formatting. It expects
// the passed configuration to already be validated, and ignores invalid options.
func configureDaemonLogs(conf *config.Config) {
	if conf.LogLevel != "" {
		lvl, err := logrus.ParseLevel(conf.LogLevel)
		if err == nil {
			logrus.SetLevel(lvl)
		}
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: jsonmessage.RFC3339NanoFixed,
		DisableColors:   conf.RawLogs,
		FullTimestamp:   true,
	})
}

func configureProxyEnv(conf *config.Config) {
	if p := conf.HTTPProxy; p != "" {
		overrideProxyEnv("HTTP_PROXY", p)
		overrideProxyEnv("http_proxy", p)
	}
	if p := conf.HTTPSProxy; p != "" {
		overrideProxyEnv("HTTPS_PROXY", p)
		overrideProxyEnv("https_proxy", p)
	}
	if p := conf.NoProxy; p != "" {
		overrideProxyEnv("NO_PROXY", p)
		overrideProxyEnv("no_proxy", p)
	}
}

func overrideProxyEnv(name, val string) {
	if oldVal := os.Getenv(name); oldVal != "" && oldVal != val {
		logrus.WithFields(logrus.Fields{
			"name":      name,
			"old-value": config.MaskCredentials(oldVal),
			"new-value": config.MaskCredentials(val),
		}).Warn("overriding existing proxy variable with value from configuration")
	}
	_ = os.Setenv(name, val)
}
