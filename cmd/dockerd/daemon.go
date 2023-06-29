package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	containerddefaults "github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/log"
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
	"github.com/docker/docker/pkg/rootless"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/plugin"
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

	d               *daemon.Daemon
	authzMiddleware *authorization.Middleware // authzMiddleware enables to dynamically reload the authorization plugins

	stopOnce    sync.Once
	apiShutdown chan struct{}
}

// NewDaemonCli returns a daemon CLI
func NewDaemonCli() *DaemonCli {
	return &DaemonCli{
		apiShutdown: make(chan struct{}),
	}
}

func (cli *DaemonCli) start(opts *daemonOptions) (err error) {
	ctx := context.TODO()

	if cli.Config, err = loadDaemonCliConfig(opts); err != nil {
		return err
	}

	tlsConfig, err := newAPIServerTLSConfig(cli.Config)
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

	log.G(ctx).Info("Starting up")

	cli.configFile = &opts.configFile
	cli.flags = opts.flags

	if cli.Config.Debug {
		debug.Enable()
	}

	if cli.Config.Experimental {
		log.G(ctx).Warn("Running experimental build")
	}

	if cli.Config.IsRootless() {
		log.G(ctx).Warn("Running in rootless mode. This mode has feature limitations.")
	}
	if rootless.RunningWithRootlessKit() {
		log.G(ctx).Info("Running with RootlessKit integration")
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

	if err := system.MkdirAll(cli.Config.ExecRoot, 0o700); err != nil {
		return err
	}

	potentiallyUnderRuntimeDir := []string{cli.Config.ExecRoot}

	if cli.Pidfile != "" {
		if err = system.MkdirAll(filepath.Dir(cli.Pidfile), 0o755); err != nil {
			return errors.Wrap(err, "failed to create pidfile directory")
		}
		if err = pidfile.Write(cli.Pidfile, os.Getpid()); err != nil {
			return errors.Wrapf(err, "failed to start daemon, ensure docker is not running or delete %s", cli.Pidfile)
		}
		potentiallyUnderRuntimeDir = append(potentiallyUnderRuntimeDir, cli.Pidfile)
		defer func() {
			if err := os.Remove(cli.Pidfile); err != nil {
				log.G(ctx).Error(err)
			}
		}()
	}

	if cli.Config.IsRootless() {
		// Set sticky bit if XDG_RUNTIME_DIR is set && the file is actually under XDG_RUNTIME_DIR
		if _, err := homedir.StickRuntimeDirContents(potentiallyUnderRuntimeDir); err != nil {
			// StickRuntimeDirContents returns nil error if XDG_RUNTIME_DIR is just unset
			log.G(ctx).WithError(err).Warn("cannot set sticky bit on files under XDG_RUNTIME_DIR")
		}
	}

	lss, hosts, err := loadListeners(cli.Config, tlsConfig)
	if err != nil {
		return errors.Wrap(err, "failed to load listeners")
	}

	ctx, cancel := context.WithCancel(context.Background())
	waitForContainerDShutdown, err := cli.initContainerd(ctx)
	if waitForContainerDShutdown != nil {
		defer waitForContainerDShutdown(10 * time.Second)
	}
	if err != nil {
		cancel()
		return err
	}
	defer cancel()

	httpServer := &http.Server{
		ReadHeaderTimeout: 5 * time.Minute, // "G112: Potential Slowloris Attack (gosec)"; not a real concern for our use, so setting a long timeout.
	}
	apiShutdownCtx, apiShutdownCancel := context.WithCancel(context.Background())
	apiShutdownDone := make(chan struct{})
	trap.Trap(cli.stop)
	go func() {
		// Block until cli.stop() has been called.
		// It may have already been called, and that's okay.
		// Any httpServer.Serve() calls made after
		// httpServer.Shutdown() will return immediately,
		// which is what we want.
		<-cli.apiShutdown
		err := httpServer.Shutdown(apiShutdownCtx)
		if err != nil {
			log.G(ctx).WithError(err).Error("Error shutting down http server")
		}
		close(apiShutdownDone)
	}()
	defer func() {
		select {
		case <-cli.apiShutdown:
			// cli.stop() has been called and the daemon has completed
			// shutting down. Give the HTTP server a little more time to
			// finish handling any outstanding requests if needed.
			tmr := time.AfterFunc(5*time.Second, apiShutdownCancel)
			defer tmr.Stop()
			<-apiShutdownDone
		default:
			// cli.start() has returned without cli.stop() being called,
			// e.g. because the daemon failed to start.
			// Stop the HTTP server with no grace period.
			if closeErr := httpServer.Close(); closeErr != nil {
				log.G(ctx).WithError(closeErr).Error("Error closing http server")
			}
		}
	}()

	// Notify that the API is active, but before daemon is set up.
	preNotifyReady()

	pluginStore := plugin.NewStore()

	var apiServer apiserver.Server
	cli.authzMiddleware = initMiddlewares(&apiServer, cli.Config, pluginStore)

	d, err := daemon.NewDaemon(ctx, cli.Config, pluginStore, cli.authzMiddleware)
	if err != nil {
		return errors.Wrap(err, "failed to start daemon")
	}

	d.StoreHosts(hosts)

	// validate after NewDaemon has restored enabled plugins. Don't change order.
	if err := validateAuthzPlugins(cli.Config.AuthorizationPlugins, pluginStore); err != nil {
		return errors.Wrap(err, "failed to validate authorization plugin")
	}

	// Note that CDI is not inherrently linux-specific, there are some linux-specific assumptions / implementations in the code that
	// queries the properties of device on the host as wel as performs the injection of device nodes and their access permissions into the OCI spec.
	//
	// In order to lift this restriction the following would have to be addressed:
	// - Support needs to be added to the cdi package for injecting Windows devices: https://github.com/container-orchestrated-devices/container-device-interface/issues/28
	// - The DeviceRequests API must be extended to non-linux platforms.
	if runtime.GOOS == "linux" && cli.Config.Experimental {
		daemon.RegisterCDIDriver(
			cdi.WithSpecDirs(cli.Config.CDISpecDirs...),
		)
	}

	cli.d = d

	if err := startMetricsServer(cli.Config.MetricsAddress); err != nil {
		return errors.Wrap(err, "failed to start metrics server")
	}

	c, err := createAndStartCluster(cli, d)
	if err != nil {
		log.G(ctx).Fatalf("Error starting cluster component: %v", err)
	}

	// Restart all autostart containers which has a swarm endpoint
	// and is not yet running now that we have successfully
	// initialized the cluster.
	d.RestartSwarmContainers()

	log.G(ctx).Info("Daemon has completed initialization")

	routerCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	routerOptions, err := newRouterOptions(routerCtx, cli.Config, d)
	if err != nil {
		return err
	}
	routerOptions.cluster = c

	httpServer.Handler = apiServer.CreateMux(routerOptions.Build()...)

	go d.ProcessClusterNotifications(ctx, c.GetWatchStream())

	cli.setupConfigReloadTrap()

	// after the daemon is done setting up we can notify systemd api
	notifyReady()

	// Daemon is fully initialized. Start handling API traffic
	// and wait for serve API to complete.
	var (
		apiWG  sync.WaitGroup
		errAPI = make(chan error, 1)
	)
	for _, ls := range lss {
		apiWG.Add(1)
		go func(ls net.Listener) {
			defer apiWG.Done()
			log.G(ctx).Infof("API listen on %s", ls.Addr())
			if err := httpServer.Serve(ls); err != http.ErrServerClosed {
				log.G(ctx).WithFields(logrus.Fields{
					logrus.ErrorKey: err,
					"listener":      ls.Addr(),
				}).Error("ServeAPI error")

				select {
				case errAPI <- err:
				default:
				}
			}
		}(ls)
	}
	apiWG.Wait()
	close(errAPI)

	c.Cleanup()

	// notify systemd that we're shutting down
	notifyStopping()
	shutdownDaemon(ctx, d)

	// Stop notification processing and any background processes
	cancel()

	if err, ok := <-errAPI; ok {
		return errors.Wrap(err, "shutting down due to ServeAPI error")
	}

	log.G(ctx).Info("Daemon shutdown complete")
	return nil
}

type routerOptions struct {
	sessionManager *session.Manager
	buildBackend   *buildbackend.Backend
	features       func() map[string]bool
	buildkit       *buildkit.Builder
	daemon         *daemon.Daemon
	cluster        *cluster.Cluster
}

func newRouterOptions(ctx context.Context, config *config.Config, d *daemon.Daemon) (routerOptions, error) {
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
		features:       d.Features,
		daemon:         d,
	}

	bk, err := buildkit.New(ctx, buildkit.Opt{
		SessionManager:      sm,
		Root:                filepath.Join(config.Root, "buildkit"),
		EngineID:            d.ID(),
		Dist:                d.DistributionServices(),
		ImageTagger:         d.ImageService(),
		NetworkController:   d.NetworkController(),
		DefaultCgroupParent: cgroupParent,
		RegistryHosts:       d.RegistryHosts,
		BuilderConfig:       config.Builder,
		Rootless:            daemon.Rootless(config),
		IdentityMapping:     d.IdentityMapping(),
		DNSConfig:           config.DNSConfig,
		ApparmorProfile:     daemon.DefaultApparmorProfile(),
		UseSnapshotter:      d.UsesSnapshotter(),
		Snapshotter:         d.ImageService().StorageDriver(),
		ContainerdAddress:   config.ContainerdAddr,
		ContainerdNamespace: config.ContainerdNamespace,
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

	return ro, nil
}

func (cli *DaemonCli) reloadConfig() {
	ctx := context.TODO()
	reload := func(c *config.Config) {
		if err := validateAuthzPlugins(c.AuthorizationPlugins, cli.d.PluginStore); err != nil {
			log.G(ctx).Fatalf("Error validating authorization plugin: %v", err)
			return
		}

		if err := cli.d.Reload(c); err != nil {
			log.G(ctx).Errorf("Error reconfiguring the daemon: %v", err)
			return
		}

		// Apply our own configuration only after the daemon reload has succeeded. We
		// don't want to partially apply the config if the daemon is unhappy with it.

		cli.authzMiddleware.SetPlugins(c.AuthorizationPlugins)

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
		log.G(ctx).Error(err)
	}
}

func (cli *DaemonCli) stop() {
	// Signal that the API server should shut down as soon as possible.
	// This construct is used rather than directly shutting down the HTTP
	// server to avoid any issues if this method is called before the server
	// has been instantiated in cli.start(). If this method is called first,
	// the HTTP server will be shut down immediately upon instantiation.
	cli.stopOnce.Do(func() {
		close(cli.apiShutdown)
	})
}

// shutdownDaemon just wraps daemon.Shutdown() to handle a timeout in case
// d.Shutdown() is waiting too long to kill container or worst it's
// blocked there
func shutdownDaemon(ctx context.Context, d *daemon.Daemon) {
	var cancel context.CancelFunc
	if timeout := d.ShutdownTimeout(); timeout >= 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}

	go func() {
		defer cancel()
		d.Shutdown(ctx)
	}()

	<-ctx.Done()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		log.G(ctx).Error("Force shutdown daemon")
	} else {
		log.G(ctx).Debug("Clean shutdown succeeded")
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

	if flags.Changed(FlagTLS) {
		conf.TLS = &opts.TLS
	}
	if flags.Changed(FlagTLSVerify) {
		conf.TLSVerify = &opts.TLSVerify
		v := true
		conf.TLS = &v
	}

	if opts.TLSOptions != nil {
		conf.TLSOptions = config.TLSOptions{
			CAFile:   opts.TLSOptions.CAFile,
			CertFile: opts.TLSOptions.CertFile,
			KeyFile:  opts.TLSOptions.KeyFile,
		}
	} else {
		conf.TLSOptions = config.TLSOptions{}
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

func (opts routerOptions) Build() []router.Router {
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
			opts.daemon.RegistryService(),
			opts.daemon.ReferenceStore,
			opts.daemon.ImageService().DistributionServices().ImageStore,
			opts.daemon.ImageService().DistributionServices().LayerStore,
		),
		systemrouter.NewRouter(opts.daemon, opts.cluster, opts.buildkit, opts.daemon.Features),
		volume.NewRouter(opts.daemon.VolumesService(), opts.cluster),
		build.NewRouter(opts.buildBackend, opts.daemon),
		sessionrouter.NewRouter(opts.sessionManager),
		swarmrouter.NewRouter(opts.cluster),
		pluginrouter.NewRouter(opts.daemon.PluginManager()),
		distributionrouter.NewRouter(opts.daemon.ImageBackend()),
	}

	if opts.buildBackend != nil {
		routers = append(routers, grpcrouter.NewRouter(opts.buildBackend))
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

	return routers
}

func initMiddlewares(s *apiserver.Server, cfg *config.Config, pluginStore plugingetter.PluginGetter) *authorization.Middleware {
	v := dockerversion.Version

	exp := middleware.NewExperimentalMiddleware(cfg.Experimental)
	s.UseMiddleware(exp)

	vm := middleware.NewVersionMiddleware(v, api.DefaultVersion, api.MinVersion)
	s.UseMiddleware(vm)

	if cfg.CorsHeaders != "" {
		c := middleware.NewCORSMiddleware(cfg.CorsHeaders)
		s.UseMiddleware(c)
	}

	authzMiddleware := authorization.NewMiddleware(cfg.AuthorizationPlugins, pluginStore)
	s.UseMiddleware(authzMiddleware)
	return authzMiddleware
}

func (cli *DaemonCli) getContainerdDaemonOpts() ([]supervisor.DaemonOpt, error) {
	var opts []supervisor.DaemonOpt
	if cli.Debug {
		opts = append(opts, supervisor.WithLogLevel("debug"))
	} else {
		opts = append(opts, supervisor.WithLogLevel(cli.LogLevel))
	}

	if !cli.CriContainerd {
		// CRI support in the managed daemon is currently opt-in.
		//
		// It's disabled by default, originally because it was listening on
		// a TCP connection at 0.0.0.0:10010, which was considered a security
		// risk, and could conflict with user's container ports.
		//
		// Current versions of containerd started now listen on localhost on
		// an ephemeral port instead, but could still conflict with container
		// ports, and running kubernetes using the static binaries is not a
		// common scenario, so we (for now) continue disabling it by default.
		//
		// Also see https://github.com/containerd/containerd/issues/2483#issuecomment-407530608
		opts = append(opts, supervisor.WithCRIDisabled())
	}

	return opts, nil
}

func newAPIServerTLSConfig(config *config.Config) (*tls.Config, error) {
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
			CAFile:             config.TLSOptions.CAFile,
			CertFile:           config.TLSOptions.CertFile,
			KeyFile:            config.TLSOptions.KeyFile,
			ExclusiveRootPools: true,
			ClientAuth:         clientAuth,
		})
		if err != nil {
			return nil, errors.Wrap(err, "invalid TLS configuration")
		}
	}

	return tlsConfig, nil
}

// checkTLSAuthOK checks basically for an explicitly disabled TLS/TLSVerify
// Going forward we do not want to support a scenario where dockerd listens
// on TCP without either TLS client auth (or an explicit opt-in to disable it)
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

func loadListeners(cfg *config.Config, tlsConfig *tls.Config) ([]net.Listener, []string, error) {
	ctx := context.TODO()

	if len(cfg.Hosts) == 0 {
		return nil, nil, errors.New("no hosts configured")
	}
	var (
		hosts []string
		lss   []net.Listener
	)

	for i := 0; i < len(cfg.Hosts); i++ {
		protoAddr := cfg.Hosts[i]
		proto, addr, ok := strings.Cut(protoAddr, "://")
		if !ok {
			return nil, nil, fmt.Errorf("bad format %s, expected PROTO://ADDR", protoAddr)
		}

		// It's a bad idea to bind to TCP without tlsverify.
		authEnabled := tlsConfig != nil && tlsConfig.ClientAuth == tls.RequireAndVerifyClientCert
		if proto == "tcp" && !authEnabled {
			log.G(ctx).WithField("host", protoAddr).Warn("Binding to IP address without --tlsverify is insecure and gives root access on this machine to everyone who has access to your network.")
			log.G(ctx).WithField("host", protoAddr).Warn("Binding to an IP address, even on localhost, can also give access to scripts run in a browser. Be safe out there!")
			time.Sleep(time.Second)

			// If TLSVerify is explicitly set to false we'll take that as "Please let me shoot myself in the foot"
			// We do not want to continue to support a default mode where tls verification is disabled, so we do some extra warnings here and eventually remove support
			if !checkTLSAuthOK(cfg) {
				ipAddr, _, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, nil, errors.Wrap(err, "error parsing tcp address")
				}

				// shortcut all this extra stuff for literal "localhost"
				// -H supports specifying hostnames, since we want to bypass this on loopback interfaces we'll look it up here.
				if ipAddr != "localhost" {
					ip := net.ParseIP(ipAddr)
					if ip == nil {
						ipA, err := net.ResolveIPAddr("ip", ipAddr)
						if err != nil {
							log.G(ctx).WithError(err).WithField("host", ipAddr).Error("Error looking up specified host address")
						}
						if ipA != nil {
							ip = ipA.IP
						}
					}
					if ip == nil || !ip.IsLoopback() {
						log.G(ctx).WithField("host", protoAddr).Warn("Binding to an IP address without --tlsverify is deprecated. Startup is intentionally being slowed down to show this message")
						log.G(ctx).WithField("host", protoAddr).Warn("Please consider generating tls certificates with client validation to prevent exposing unauthenticated root access to your network")
						log.G(ctx).WithField("host", protoAddr).Warnf("You can override this by explicitly specifying '--%s=false' or '--%s=false'", FlagTLS, FlagTLSVerify)
						log.G(ctx).WithField("host", protoAddr).Warnf("Support for listening on TCP without authentication or explicit intent to run without authentication will be removed in the next release")

						time.Sleep(15 * time.Second)
					}
				}
			}
		}
		// If we're binding to a TCP port, make sure that a container doesn't try to use it.
		if proto == "tcp" {
			if err := allocateDaemonPort(addr); err != nil {
				return nil, nil, err
			}
		}
		ls, err := listeners.Init(proto, addr, cfg.SocketGroup, tlsConfig)
		if err != nil {
			return nil, nil, err
		}
		log.G(ctx).Debugf("Listener created for HTTP on %s (%s)", proto, addr)
		hosts = append(hosts, addr)
		lss = append(lss, ls...)
	}

	return lss, hosts, nil
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
		ImageBackend:           d.ImageBackend(),
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
		log.G(context.TODO()).WithFields(logrus.Fields{
			"name":      name,
			"old-value": config.MaskCredentials(oldVal),
			"new-value": config.MaskCredentials(val),
		}).Warn("overriding existing proxy variable with value from configuration")
	}
	_ = os.Setenv(name, val)
}
