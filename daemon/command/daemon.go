package command

import (
	"context"
	"crypto/tls"
	"encoding/json"
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

	containerddefaults "github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/containerd/v2/pkg/tracing"
	"github.com/containerd/log"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/tracing/detect"
	"github.com/moby/moby/v2/daemon"
	buildbackend "github.com/moby/moby/v2/daemon/builder/backend"
	"github.com/moby/moby/v2/daemon/builder/dockerfile"
	"github.com/moby/moby/v2/daemon/cluster"
	"github.com/moby/moby/v2/daemon/command/debug"
	"github.com/moby/moby/v2/daemon/command/trap"
	"github.com/moby/moby/v2/daemon/config"
	buildkit "github.com/moby/moby/v2/daemon/internal/builder-next"
	"github.com/moby/moby/v2/daemon/internal/builder-next/exporter"
	"github.com/moby/moby/v2/daemon/internal/libcontainerd/supervisor"
	"github.com/moby/moby/v2/daemon/internal/otelutil"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	"github.com/moby/moby/v2/daemon/listeners"
	dopts "github.com/moby/moby/v2/daemon/pkg/opts"
	"github.com/moby/moby/v2/daemon/pkg/plugin"
	apiserver "github.com/moby/moby/v2/daemon/server"
	"github.com/moby/moby/v2/daemon/server/middleware"
	"github.com/moby/moby/v2/daemon/server/router"
	"github.com/moby/moby/v2/daemon/server/router/build"
	checkpointrouter "github.com/moby/moby/v2/daemon/server/router/checkpoint"
	"github.com/moby/moby/v2/daemon/server/router/container"
	debugrouter "github.com/moby/moby/v2/daemon/server/router/debug"
	distributionrouter "github.com/moby/moby/v2/daemon/server/router/distribution"
	grpcrouter "github.com/moby/moby/v2/daemon/server/router/grpc" //nolint:staticcheck // Deprecated endpoint kept for backward compatibility
	"github.com/moby/moby/v2/daemon/server/router/image"
	"github.com/moby/moby/v2/daemon/server/router/network"
	pluginrouter "github.com/moby/moby/v2/daemon/server/router/plugin"
	sessionrouter "github.com/moby/moby/v2/daemon/server/router/session"
	swarmrouter "github.com/moby/moby/v2/daemon/server/router/swarm"
	systemrouter "github.com/moby/moby/v2/daemon/server/router/system"
	"github.com/moby/moby/v2/daemon/server/router/volume"
	"github.com/moby/moby/v2/dockerversion"
	"github.com/moby/moby/v2/pkg/authorization"
	"github.com/moby/moby/v2/pkg/homedir"
	"github.com/moby/moby/v2/pkg/pidfile"
	"github.com/moby/moby/v2/pkg/plugingetter"
	"github.com/moby/sys/userns"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/bridge/opencensus"
	"go.opentelemetry.io/otel/propagation"
	"tags.cncf.io/container-device-interface/pkg/cdi"
)

// daemonCLI represents the daemon CLI.
type daemonCLI struct {
	*config.Config
	configFile *string
	flags      *pflag.FlagSet

	d               *daemon.Daemon
	authzMiddleware *authorization.Middleware // authzMiddleware enables to dynamically reload the authorization plugins

	stopOnce     sync.Once
	apiShutdown  chan struct{}
	apiTLSConfig *tls.Config
}

// newDaemonCLI returns a daemon CLI with the given options.
func newDaemonCLI(opts *daemonOptions) (*daemonCLI, error) {
	cfg, err := loadDaemonCliConfig(opts)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := newAPIServerTLSConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &daemonCLI{
		Config:       cfg,
		configFile:   &opts.configFile,
		flags:        opts.flags,
		apiShutdown:  make(chan struct{}),
		apiTLSConfig: tlsConfig,
	}, nil
}

func (cli *daemonCLI) start(ctx context.Context) (err error) {
	configureProxyEnv(ctx, cli.Config.Proxies)
	if err := configureDaemonLogs(ctx, cli.Config.DaemonLogConfig); err != nil {
		return fmt.Errorf("failed to configure daemon logging: %w", err)
	}

	log.G(ctx).Info("Starting up")

	if cli.Config.Debug {
		debug.Enable()
	}

	if rootless.RunningWithRootlessKit() && !cli.Config.IsRootless() {
		return errors.New("rootless mode needs to be enabled for running with RootlessKit")
	}

	// return human-friendly error before creating files
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		return errors.New("dockerd needs to be started with root privileges. To run dockerd in rootless mode as an unprivileged user, see https://docs.docker.com/go/rootless/")
	}

	if err := setDefaultUmask(); err != nil {
		return err
	}

	// Create the daemon root before we create ANY other files (PID, or migrate keys)
	// to ensure the appropriate ACL is set (particularly relevant on Windows)
	if err := daemon.CreateDaemonRoot(cli.Config); err != nil {
		return err
	}

	if err := os.MkdirAll(cli.Config.ExecRoot, 0o700); err != nil {
		return err
	}

	potentiallyUnderRuntimeDir := []string{cli.Config.ExecRoot}

	if cli.Pidfile != "" {
		if err = os.MkdirAll(filepath.Dir(cli.Pidfile), 0o755); err != nil {
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
		log.G(ctx).Warn("Running in rootless mode. This mode has feature limitations.")
		if rootless.RunningWithRootlessKit() {
			log.G(ctx).Info("Running with RootlessKit integration")
		}

		// Set sticky bit if XDG_RUNTIME_DIR is set && the file is actually under XDG_RUNTIME_DIR
		if _, err := homedir.StickRuntimeDirContents(potentiallyUnderRuntimeDir); err != nil {
			// StickRuntimeDirContents returns nil error if XDG_RUNTIME_DIR is just unset
			log.G(ctx).WithError(err).Warn("cannot set sticky bit on files under XDG_RUNTIME_DIR")
		}
	}
	if cli.Config.Experimental {
		log.G(ctx).Warn("Running with experimental features enabled")
	}

	lss, hosts, err := loadListeners(cli.Config, cli.apiTLSConfig)
	if err != nil {
		return errors.Wrap(err, "failed to load listeners")
	}

	ctx, cancel := context.WithCancel(ctx)
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
	apiShutdownCtx, apiShutdownCancel := context.WithCancel(context.WithoutCancel(ctx))
	apiShutdownDone := make(chan struct{})
	trap.Trap(cli.stop)
	go func() {
		// Block until cli.stop() has been called.
		// It may have already been called, and that's okay.
		// Any httpServer.Serve() calls made after
		// httpServer.Shutdown() will return immediately,
		// which is what we want.
		<-cli.apiShutdown
		if err := httpServer.Shutdown(apiShutdownCtx); err != nil {
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
			if err := httpServer.Close(); err != nil {
				log.G(ctx).WithError(err).Error("Error closing http server")
			}
		}
	}()

	// Notify that the API is active, but before daemon is set up.
	if err := preNotifyReady(); err != nil {
		return err
	}

	const otelServiceNameEnv = "OTEL_SERVICE_NAME"
	if _, ok := os.LookupEnv(otelServiceNameEnv); !ok {
		_ = os.Setenv(otelServiceNameEnv, filepath.Base(os.Args[0]))
	}

	setOTLPProtoDefault()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// Initialize the trace recorder for buildkit.
	detect.Recorder = detect.NewTraceRecorder()

	tp, otelShutdown := otelutil.NewTracerProvider(ctx, true)
	otel.SetTracerProvider(tp)
	log.G(ctx).Logger.AddHook(tracing.NewLogrusHook())
	// The github.com/microsoft/hcsshim module is instrumented with
	// OpenCensus, but we use OpenTelemetry for tracing in the daemon.
	// Bridge OpenCensus to OpenTelemetry so OC trace spans are exported to
	// the daemon's configured OTEL collector.
	opencensus.InstallTraceBridge()

	pluginStore := plugin.NewStore()

	// Register the CDI driver before the daemon starts, as it might try to restore containers that depend on the CDI driver.
	// Note that CDI is not inherently linux-specific, there are some linux-specific assumptions / implementations in the code that
	// queries the properties of device on the host as well as performs the injection of device nodes and their access permissions into the OCI spec.
	//
	// In order to lift this restriction the following would have to be addressed:
	// - Support needs to be added to the cdi package for injecting Windows devices: https://tags.cncf.io/container-device-interface/issues/28
	// - The DeviceRequests API must be extended to non-linux platforms.
	var cdiCache *cdi.Cache
	if cdiEnabled(cli.Config) {
		cdiCache = daemon.RegisterCDIDriver(cli.Config.CDISpecDirs...)
	}

	var apiServer apiserver.Server
	cli.authzMiddleware, err = initMiddlewares(ctx, &apiServer, cli.Config, pluginStore)
	if err != nil {
		return errors.Wrap(err, "failed to start API server")
	}

	d, err := daemon.NewDaemon(ctx, cli.Config, pluginStore, cli.authzMiddleware)
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

	c, err := createAndStartCluster(d, cli.Config)
	if err != nil {
		return fmt.Errorf("failed to start cluster component: %w", err)
	}

	// Restart all autostart containers which has a swarm endpoint
	// and is not yet running now that we have successfully
	// initialized the cluster.
	d.RestartSwarmContainers()

	b, shutdownBuildKit, err := initBuildkit(ctx, d, cdiCache)
	if err != nil {
		return fmt.Errorf("error initializing buildkit: %w", err)
	}

	if runtime.GOOS == "windows" {
		if enabled, ok := d.Features()["buildkit"]; ok && enabled {
			log.G(ctx).Warn("Buildkit feature is enabled in the daemon.json configuration file. Support for BuildKit on Windows is experimental, and enabling this feature may not work. Use at your own risk!")
		}
	}

	// Enable HTTP/1, HTTP/2 and h2c on the HTTP server. h2c won't be used for *tls.Conn listeners, and HTTP/2 won't be
	// used for non-TLS connections.
	var p http.Protocols
	p.SetHTTP1(true)
	p.SetHTTP2(true)
	p.SetUnencryptedHTTP2(true)

	routers := buildRouters(routerOptions{
		features: d.Features,
		daemon:   d,
		cluster:  c,
		builder:  b,
	})
	gs := newGRPCServer(ctx)
	b.backend.RegisterGRPC(gs)
	httpServer.Protocols = &p
	httpServer.Handler = newHTTPHandler(ctx, gs, apiServer.CreateMux(ctx, routers...))

	go d.ProcessClusterNotifications(ctx, c.GetWatchStream())

	cli.setupConfigReloadTrap()

	// after the daemon is done setting up we can notify systemd api
	notifyReady()
	log.G(ctx).Info("Daemon has completed initialization")

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
				log.G(ctx).WithFields(log.Fields{
					"error":    err,
					"listener": ls.Addr(),
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

	// shutdown / close BuildKit backend
	shutdownBuildKit()

	// Stop notification processing and any background processes
	cancel()

	if err, ok := <-errAPI; ok {
		return errors.Wrap(err, "shutting down due to ServeAPI error")
	}

	if err := otelShutdown(context.WithoutCancel(ctx)); err != nil {
		log.G(ctx).WithError(err).Error("Failed to shutdown OTEL tracing")
	}

	log.G(ctx).Info("Daemon shutdown complete")
	return nil
}

// The buildkit "detect" package uses grpc as the default proto, which is in conformance with the old spec.
// For a little while now http/protobuf is the default spec, so this function sets the protocol to http/protobuf when the env var is unset
// so that the detect package will use http/protobuf as a default.
// TODO: This can be removed after buildkit is updated to use http/protobuf as the default.
func setOTLPProtoDefault() {
	const (
		tracesEnv  = "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL"
		metricsEnv = "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL"
		protoEnv   = "OTEL_EXPORTER_OTLP_PROTOCOL"

		defaultProto = "http/protobuf"
	)

	if os.Getenv(protoEnv) == "" {
		if os.Getenv(tracesEnv) == "" {
			_ = os.Setenv(tracesEnv, defaultProto)
		}
		if os.Getenv(metricsEnv) == "" {
			_ = os.Setenv(metricsEnv, defaultProto)
		}
	}
}

func initBuildkit(ctx context.Context, d *daemon.Daemon, cdiCache *cdi.Cache) (_ builderOptions, closeFn func(), _ error) {
	log.G(ctx).Info("Initializing buildkit")
	closeFn = func() {}

	sm, err := session.NewManager()
	if err != nil {
		return builderOptions{}, closeFn, errors.Wrap(err, "failed to create sessionmanager")
	}

	manager, err := dockerfile.NewBuildManager(d.BuilderBackend(), d.IdentityMapping())
	if err != nil {
		return builderOptions{}, closeFn, err
	}

	cfg := d.Config()

	bk, err := buildkit.New(ctx, buildkit.Opt{
		SessionManager:      sm,
		Root:                filepath.Join(cfg.Root, "buildkit"),
		EngineID:            d.ID(),
		Dist:                d.DistributionServices(),
		ImageTagger:         d.ImageService(),
		NetworkController:   d.NetworkController(),
		DefaultCgroupParent: newCgroupParent(&cfg),
		RegistryHosts:       d.RegistryHosts,
		BuilderConfig:       cfg.Builder,
		Rootless:            daemon.Rootless(&cfg),
		IdentityMapping:     d.IdentityMapping(),
		DNSConfig:           cfg.DNSConfig,
		ApparmorProfile:     daemon.DefaultApparmorProfile(),
		UseSnapshotter:      d.UsesSnapshotter(),
		Snapshotter:         d.ImageService().StorageDriver(),
		ContainerdAddress:   cfg.ContainerdAddr,
		ContainerdNamespace: cfg.ContainerdNamespace,
		HyperVIsolation:     d.DefaultIsolation().IsHyperV(),
		Callbacks: exporter.BuildkitCallbacks{
			Exported: d.ImageExportedByBuildkit,
			Named:    d.ImageNamedByBuildkit,
		},
		CDICache: cdiCache,
	})
	if err != nil {
		return builderOptions{}, closeFn, errors.Wrap(err, "error creating buildkit instance")
	}

	bb, err := buildbackend.NewBackend(d.ImageService(), manager, bk, d.EventsService)
	if err != nil {
		return builderOptions{}, closeFn, errors.Wrap(err, "failed to create builder backend")
	}

	log.G(ctx).Info("Completed buildkit initialization")

	closeFn = func() {
		if err := bk.Close(); err != nil {
			log.G(ctx).WithError(err).Error("Failed to close buildkit")
		}
	}

	return builderOptions{
		backend:        bb,
		buildkit:       bk,
		sessionManager: sm,
	}, closeFn, nil
}

type routerOptions struct {
	features func() map[string]bool
	daemon   *daemon.Daemon
	cluster  *cluster.Cluster
	builder  builderOptions
}

type builderOptions struct {
	backend        *buildbackend.Backend
	buildkit       *buildkit.Builder
	sessionManager *session.Manager
}

func (cli *daemonCLI) reloadConfig() {
	ctx := context.TODO()
	log.G(ctx).WithField("config-file", *cli.configFile).Info("Got signal to reload configuration")
	reload := func(cfg *config.Config) {
		if err := validateAuthzPlugins(cfg.AuthorizationPlugins, cli.d.PluginStore); err != nil {
			log.G(ctx).WithError(err).Fatal("Error validating authorization plugin")
			return
		}

		if err := cli.d.Reload(cfg); err != nil {
			log.G(ctx).WithError(err).Error("Error reconfiguring the daemon")
			return
		}

		// Apply our own configuration only after the daemon reload has succeeded. We
		// don't want to partially apply the config if the daemon is unhappy with it.

		cli.authzMiddleware.SetPlugins(cfg.AuthorizationPlugins)

		if cfg.IsValueSet("debug") {
			debugEnabled := debug.IsEnabled()
			switch {
			case debugEnabled && !cfg.Debug: // disable debug
				debug.Disable()
			case cfg.Debug && !debugEnabled: // enable debug
				debug.Enable()
			}
		}
	}

	if err := config.Reload(*cli.configFile, cli.flags, reload); err != nil {
		log.G(ctx).WithError(err).Error("Error reloading configuration")
		return
	}

	sanitizedConfig := config.Sanitize(cli.d.Config())
	jsonData, err := json.Marshal(sanitizedConfig)
	if err != nil {
		log.G(context.TODO()).WithError(err).Warn("Error when marshaling configuration for printing")
		log.G(context.TODO()).Info("Reloaded configuration")
	} else {
		log.G(context.TODO()).WithField("config", string(jsonData)).Info("Reloaded configuration")
	}
}

func (cli *daemonCLI) stop() {
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

	// The DOCKER_MIN_API_VERSION env-var allows overriding the minimum API
	// version provided by the daemon within constraints of the minimum and
	// maximum (current) supported API versions.
	//
	// API versions older than [config.defaultMinAPIVersion] are deprecated and
	// to be removed in a future release. The "DOCKER_MIN_API_VERSION" env-var
	// should only be used for exceptional cases.
	if ver := os.Getenv("DOCKER_MIN_API_VERSION"); ver != "" {
		if err := config.ValidateMinAPIVersion(ver); err != nil {
			return nil, errors.Wrap(err, "invalid DOCKER_MIN_API_VERSION")
		}
		conf.MinAPIVersion = ver
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
		conf.TLSOptions = config.TLSOptions{
			CAFile:   opts.TLSOptions.CAFile,
			CertFile: opts.TLSOptions.CertFile,
			KeyFile:  opts.TLSOptions.KeyFile,
		}
	} else {
		conf.TLSOptions = config.TLSOptions{}
	}

	if runtime.GOOS == "windows" && opts.configFile == "" {
		// On Windows, the location of the config-file is relative to the
		// daemon's data-root, which is configurable, so we cannot use a
		// fixed default location. Instead, we set the location here.
		//
		// FIXME(thaJeztah): find a better default on Windows that does not depend on "daemon.json" or the --data-root option.
		opts.configFile = filepath.Join(conf.Root, "config", "daemon.json")
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

	if conf.CDISpecDirs == nil {
		// If the CDISpecDirs is not set at this stage, we set it to the default.
		conf.CDISpecDirs = append([]string(nil), cdi.DefaultSpecDirs...)
		if rootless.RunningWithRootlessKit() {
			// In rootless mode, we add the user-specific CDI spec directory.
			xch, err := homedir.GetConfigHome()
			if err != nil {
				return nil, err
			}
			xrd, err := homedir.GetRuntimeDir()
			if err != nil {
				return nil, err
			}
			conf.CDISpecDirs = append(conf.CDISpecDirs, filepath.Join(xch, "cdi"), filepath.Join(xrd, "cdi"))
		}
	}
	// Filter out CDI spec directories that are not readable, and log appropriately
	var cdiSpecDirs []string
	for _, dir := range conf.CDISpecDirs {
		// Non-existing directories are not filtered out here, as CDI spec directories are allowed to not exist.
		if _, err := os.ReadDir(dir); err == nil || errors.Is(err, os.ErrNotExist) {
			cdiSpecDirs = append(cdiSpecDirs, dir)
		} else {
			logLevel := log.ErrorLevel
			if userns.RunningInUserNS() && errors.Is(err, os.ErrPermission) {
				logLevel = log.DebugLevel
			}
			log.L.WithField("dir", dir).WithError(err).Log(logLevel, "CDI spec directory cannot be accessed, skipping")
		}
	}
	conf.CDISpecDirs = cdiSpecDirs
	if len(conf.CDISpecDirs) == 1 && conf.CDISpecDirs[0] == "" {
		// If CDISpecDirs is set to an empty string, we clear it to ensure that CDI is disabled.
		conf.CDISpecDirs = nil
	}
	// Only clear CDISpecDirs if CDI is explicitly disabled
	if val, exists := conf.Features["cdi"]; exists && !val {
		// If the CDI feature is explicitly disabled, we clear the CDISpecDirs to ensure that CDI is disabled.
		conf.CDISpecDirs = nil
	}

	if err := setPlatformOptions(conf); err != nil {
		return nil, err
	}

	return conf, nil
}

// normalizeHosts normalizes the configured config.Hosts and remove duplicates.
// It returns an error if it fails to parse a host.
func normalizeHosts(cfg *config.Config) error {
	if len(cfg.Hosts) == 0 {
		// if no hosts are configured, create a single entry slice, so that the
		// default is used.
		//
		// TODO(thaJeztah) implement a cleaner way for this; this depends on a
		//                 side-effect of how we parse empty/partial hosts.
		cfg.Hosts = make([]string, 1)
	}
	hosts := make([]string, 0, len(cfg.Hosts))
	seen := make(map[string]struct{}, len(cfg.Hosts))

	useTLS := DefaultTLSValue
	if cfg.TLS != nil {
		useTLS = *cfg.TLS
	}

	for _, h := range cfg.Hosts {
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
	hosts = appendApiSocket(hosts)
	sort.Strings(hosts)
	cfg.Hosts = hosts
	return nil
}

func buildRouters(opts routerOptions) []router.Router {
	routers := []router.Router{
		// we need to add the checkpoint router before the container router or the DELETE gets masked
		checkpointrouter.NewRouter(opts.daemon),
		container.NewRouter(opts.daemon),
		image.NewRouter(
			opts.daemon.ImageService(),
			opts.daemon.RegistryService(),
		),
		systemrouter.NewRouter(opts.daemon, opts.cluster, opts.builder.buildkit, opts.daemon.Features),
		volume.NewRouter(opts.daemon.VolumesService(), opts.cluster),
		build.NewRouter(opts.builder.backend, opts.daemon),
		sessionrouter.NewRouter(opts.builder.sessionManager), //nolint:staticcheck // Deprecated endpoint kept for backward compatibility
		swarmrouter.NewRouter(opts.cluster),
		pluginrouter.NewRouter(opts.daemon.PluginManager()),
		distributionrouter.NewRouter(opts.daemon.ImageBackend()),
		network.NewRouter(opts.daemon, opts.cluster),
		debugrouter.NewRouter(),
	}

	if opts.builder.backend != nil {
		routers = append(routers, grpcrouter.NewRouter(opts.builder.backend)) //nolint:staticcheck // Deprecated endpoint kept for backward compatibility
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

func initMiddlewares(_ context.Context, s *apiserver.Server, cfg *config.Config, pluginStore plugingetter.PluginGetter) (*authorization.Middleware, error) {
	exp := middleware.NewExperimentalMiddleware(cfg.Experimental)
	s.UseMiddleware(exp)

	vm, err := middleware.NewVersionMiddleware(dockerversion.Version, config.MaxAPIVersion, cfg.MinAPIVersion)
	if err != nil {
		return nil, err
	}
	s.UseMiddleware(*vm)

	authzMiddleware := authorization.NewMiddleware(cfg.AuthorizationPlugins, pluginStore)
	s.UseMiddleware(authzMiddleware)
	return authzMiddleware, nil
}

func getContainerdDaemonOpts(cfg *config.Config) ([]supervisor.DaemonOpt, error) {
	var opts []supervisor.DaemonOpt
	if cfg.Debug {
		opts = append(opts, supervisor.WithLogLevel("debug"))
	} else {
		opts = append(opts, supervisor.WithLogLevel(cfg.DaemonLogConfig.LogLevel))
	}

	if logFormat := cfg.DaemonLogConfig.LogFormat; logFormat != "" {
		opts = append(opts, supervisor.WithLogFormat(logFormat))
	}

	if !cfg.CriContainerd {
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

	if runtime.GOOS == "windows" {
		opts = append(opts, supervisor.WithDetectLocalBinary())
	}

	return opts, nil
}

func newAPIServerTLSConfig(cfg *config.Config) (*tls.Config, error) {
	var tlsConfig *tls.Config
	if cfg.TLS != nil && *cfg.TLS {
		var (
			clientAuth tls.ClientAuthType
			err        error
		)
		if cfg.TLSVerify == nil || *cfg.TLSVerify {
			// server requires and verifies client's certificate
			clientAuth = tls.RequireAndVerifyClientCert
		}
		tlsConfig, err = tlsconfig.Server(tlsconfig.Options{
			CAFile:             cfg.TLSOptions.CAFile,
			CertFile:           cfg.TLSOptions.CertFile,
			KeyFile:            cfg.TLSOptions.KeyFile,
			ExclusiveRootPools: true,
			ClientAuth:         clientAuth,
		})
		if err != nil {
			return nil, errors.Wrap(err, "invalid TLS configuration")
		}
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	}

	return tlsConfig, nil
}

// checkTLSAuthOK checks basically for an explicitly disabled TLS/TLSVerify
// Going forward we do not want to support a scenario where dockerd listens
// on TCP without either TLS client auth (or an explicit opt-in to disable it)
func checkTLSAuthOK(cfg *config.Config) bool {
	if cfg.TLS == nil {
		// Either TLS is enabled by default, in which case TLS verification should be enabled by default, or explicitly disabled
		// Or TLS is disabled by default... in any of these cases, we can just take the default value as to how to proceed
		return DefaultTLSValue
	}

	if !*cfg.TLS {
		// TLS is explicitly disabled, which is supported
		return true
	}

	if cfg.TLSVerify == nil {
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
			log.G(ctx).WithField("host", protoAddr).Warn("[DEPRECATION NOTICE] In future versions this will be a hard failure preventing the daemon from starting! Learn more at: https://docs.docker.com/go/api-security/")
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

func createAndStartCluster(d *daemon.Daemon, cfg *config.Config) (*cluster.Cluster, error) {
	name, _ := os.Hostname()
	c, err := cluster.New(cluster.Config{
		Root:                   cfg.Root,
		Name:                   name,
		Backend:                d,
		VolumeBackend:          d.VolumesService(),
		ImageBackend:           d.ImageBackend(),
		PluginBackend:          d.PluginManager(),
		NetworkSubnetsProvider: d,
		DefaultAdvertiseAddr:   cfg.SwarmDefaultAdvertiseAddr,
		RaftHeartbeatTick:      cfg.SwarmRaftHeartbeatTick,
		RaftElectionTick:       cfg.SwarmRaftElectionTick,
		RuntimeRoot:            getSwarmRunRoot(cfg),
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

// configureDaemonLogs sets the logging level and formatting. It expects
// the passed configuration to already be validated, and ignores invalid options.
func configureDaemonLogs(ctx context.Context, conf config.DaemonLogConfig) error {
	switch conf.LogFormat {
	case log.JSONFormat:
		if err := log.SetFormat(log.JSONFormat); err != nil {
			return err
		}
	case log.TextFormat, "":
		if err := log.SetFormat(log.TextFormat); err != nil {
			return err
		}
		if conf.RawLogs {
			// FIXME(thaJeztah): this needs a better solution: containerd doesn't allow disabling colors, and this code is depending on internal knowledge of "log.SetFormat"
			if l, ok := log.L.Logger.Formatter.(*logrus.TextFormatter); ok {
				l.DisableColors = true
			}
		}
	default:
		return fmt.Errorf("unknown log format: %s", conf.LogFormat)
	}

	logLevel := conf.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	if err := log.SetLevel(logLevel); err != nil {
		log.G(ctx).WithError(err).Warn("configure log level")
	}
	return nil
}

func configureProxyEnv(ctx context.Context, cfg config.Proxies) {
	if p := cfg.HTTPProxy; p != "" {
		overrideProxyEnv(ctx, "HTTP_PROXY", p)
		overrideProxyEnv(ctx, "http_proxy", p)
	}
	if p := cfg.HTTPSProxy; p != "" {
		overrideProxyEnv(ctx, "HTTPS_PROXY", p)
		overrideProxyEnv(ctx, "https_proxy", p)
	}
	if p := cfg.NoProxy; p != "" {
		overrideProxyEnv(ctx, "NO_PROXY", p)
		overrideProxyEnv(ctx, "no_proxy", p)
	}
}

func overrideProxyEnv(ctx context.Context, name, val string) {
	if oldVal := os.Getenv(name); oldVal != "" && oldVal != val {
		log.G(ctx).WithFields(log.Fields{
			"name":      name,
			"old-value": config.MaskCredentials(oldVal),
			"new-value": config.MaskCredentials(val),
		}).Warn("overriding existing proxy variable with value from configuration")
	}
	_ = os.Setenv(name, val)
}

func (cli *daemonCLI) initializeContainerd(ctx context.Context) (func(time.Duration) error, error) {
	systemContainerdAddr, ok, err := systemContainerdRunning(honorXDG)
	if err != nil {
		return nil, errors.Wrap(err, "could not determine whether the system containerd is running")
	}
	if ok {
		// detected a system containerd at the given address.
		cli.Config.ContainerdAddr = systemContainerdAddr
		return nil, nil
	}

	log.G(ctx).Info("containerd not running, starting managed containerd")
	opts, err := getContainerdDaemonOpts(cli.Config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate containerd options")
	}

	r, err := supervisor.Start(ctx, filepath.Join(cli.Config.Root, "containerd"), filepath.Join(cli.Config.ExecRoot, "containerd"), opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start containerd")
	}
	cli.Config.ContainerdAddr = r.Address()

	// Try to wait for containerd to shutdown
	return r.WaitTimeout, nil
}

// cdiEnabled returns true if CDI feature wasn't explicitly disabled via
// features.
func cdiEnabled(conf *config.Config) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	val, ok := conf.Features["cdi"]
	if !ok {
		return true
	}
	return val
}
