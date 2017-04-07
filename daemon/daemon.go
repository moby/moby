// Package daemon exposes the functions that occur on the host server
// that the Docker daemon is running.
//
// In implementing the various functions of the daemon, there is often
// a method-specific struct for configuring the runtime behavior.
package daemon

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	containerd "github.com/docker/containerd/api/grpc/types"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/discovery"
	"github.com/docker/docker/daemon/events"
	"github.com/docker/docker/daemon/exec"
	// register graph drivers
	_ "github.com/docker/docker/daemon/graphdriver/register"
	"github.com/docker/docker/daemon/initlayer"
	"github.com/docker/docker/daemon/stats"
	dmetadata "github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/migrate/v1"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/registrar"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/pkg/truncindex"
	"github.com/docker/docker/plugin"
	refstore "github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	volumedrivers "github.com/docker/docker/volume/drivers"
	"github.com/docker/docker/volume/local"
	"github.com/docker/docker/volume/store"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/cluster"
	nwconfig "github.com/docker/libnetwork/config"
	"github.com/docker/libtrust"
	"github.com/pkg/errors"
)

var (
	// DefaultRuntimeBinary is the default runtime to be used by
	// containerd if none is specified
	DefaultRuntimeBinary = "docker-runc"

	// DefaultInitBinary is the name of the default init binary
	DefaultInitBinary = "docker-init"

	errSystemNotSupported = errors.New("The Docker daemon is not supported on this platform.")
)

// Daemon holds information about the Docker daemon.
type Daemon struct {
	ID                        string
	repository                string
	containers                container.Store
	execCommands              *exec.Store
	referenceStore            refstore.Store
	downloadManager           *xfer.LayerDownloadManager
	uploadManager             *xfer.LayerUploadManager
	distributionMetadataStore dmetadata.Store
	trustKey                  libtrust.PrivateKey
	idIndex                   *truncindex.TruncIndex
	configStore               *config.Config
	statsCollector            *stats.Collector
	defaultLogConfig          containertypes.LogConfig
	RegistryService           registry.Service
	EventsService             *events.Events
	netController             libnetwork.NetworkController
	volumes                   *store.VolumeStore
	discoveryWatcher          discovery.Reloader
	root                      string
	seccompEnabled            bool
	apparmorEnabled           bool
	shutdown                  bool
	uidMaps                   []idtools.IDMap
	gidMaps                   []idtools.IDMap
	layerStore                layer.Store
	imageStore                image.Store
	PluginStore               *plugin.Store // todo: remove
	pluginManager             *plugin.Manager
	nameIndex                 *registrar.Registrar
	linkIndex                 *linkIndex
	containerd                libcontainerd.Client
	containerdRemote          libcontainerd.Remote
	defaultIsolation          containertypes.Isolation // Default isolation mode on Windows
	clusterProvider           cluster.Provider
	cluster                   Cluster

	machineMemory uint64

	seccompProfile     []byte
	seccompProfilePath string
}

// HasExperimental returns whether the experimental features of the daemon are enabled or not
func (daemon *Daemon) HasExperimental() bool {
	if daemon.configStore != nil && daemon.configStore.Experimental {
		return true
	}
	return false
}

func (daemon *Daemon) restore() error {
	var (
		currentDriver = daemon.GraphDriverName()
		containers    = make(map[string]*container.Container)
	)

	logrus.Info("Loading containers: start.")

	dir, err := ioutil.ReadDir(daemon.repository)
	if err != nil {
		return err
	}

	for _, v := range dir {
		id := v.Name()
		container, err := daemon.load(id)
		if err != nil {
			logrus.Errorf("Failed to load container %v: %v", id, err)
			continue
		}

		// Ignore the container if it does not support the current driver being used by the graph
		if (container.Driver == "" && currentDriver == "aufs") || container.Driver == currentDriver {
			rwlayer, err := daemon.layerStore.GetRWLayer(container.ID)
			if err != nil {
				logrus.Errorf("Failed to load container mount %v: %v", id, err)
				continue
			}
			container.RWLayer = rwlayer
			logrus.Debugf("Loaded container %v", container.ID)

			containers[container.ID] = container
		} else {
			logrus.Debugf("Cannot load container %s because it was created with another graph driver.", container.ID)
		}
	}

	removeContainers := make(map[string]*container.Container)
	restartContainers := make(map[*container.Container]chan struct{})
	activeSandboxes := make(map[string]interface{})
	for id, c := range containers {
		if err := daemon.registerName(c); err != nil {
			logrus.Errorf("Failed to register container %s: %s", c.ID, err)
			delete(containers, id)
			continue
		}
		daemon.Register(c)

		// verify that all volumes valid and have been migrated from the pre-1.7 layout
		if err := daemon.verifyVolumesInfo(c); err != nil {
			// don't skip the container due to error
			logrus.Errorf("Failed to verify volumes for container '%s': %v", c.ID, err)
		}

		// The LogConfig.Type is empty if the container was created before docker 1.12 with default log driver.
		// We should rewrite it to use the daemon defaults.
		// Fixes https://github.com/docker/docker/issues/22536
		if c.HostConfig.LogConfig.Type == "" {
			if err := daemon.mergeAndVerifyLogConfig(&c.HostConfig.LogConfig); err != nil {
				logrus.Errorf("Failed to verify log config for container %s: %q", c.ID, err)
				continue
			}
		}
	}

	var wg sync.WaitGroup
	var mapLock sync.Mutex
	for _, c := range containers {
		wg.Add(1)
		go func(c *container.Container) {
			defer wg.Done()
			if err := backportMountSpec(c); err != nil {
				logrus.Error("Failed to migrate old mounts to use new spec format")
			}

			if c.IsRunning() || c.IsPaused() {
				c.RestartManager().Cancel() // manually start containers because some need to wait for swarm networking
				if err := daemon.containerd.Restore(c.ID, c.InitializeStdio); err != nil {
					logrus.Errorf("Failed to restore %s with containerd: %s", c.ID, err)
					return
				}

				// we call Mount and then Unmount to get BaseFs of the container
				if err := daemon.Mount(c); err != nil {
					// The mount is unlikely to fail. However, in case mount fails
					// the container should be allowed to restore here. Some functionalities
					// (like docker exec -u user) might be missing but container is able to be
					// stopped/restarted/removed.
					// See #29365 for related information.
					// The error is only logged here.
					logrus.Warnf("Failed to mount container on getting BaseFs path %v: %v", c.ID, err)
				} else {
					// if mount success, then unmount it
					if err := daemon.Unmount(c); err != nil {
						logrus.Warnf("Failed to umount container on getting BaseFs path %v: %v", c.ID, err)
					}
				}

				c.ResetRestartManager(false)
				if !c.HostConfig.NetworkMode.IsContainer() && c.IsRunning() {
					options, err := daemon.buildSandboxOptions(c)
					if err != nil {
						logrus.Warnf("Failed build sandbox option to restore container %s: %v", c.ID, err)
					}
					mapLock.Lock()
					activeSandboxes[c.NetworkSettings.SandboxID] = options
					mapLock.Unlock()
				}

			}
			// fixme: only if not running
			// get list of containers we need to restart
			if !c.IsRunning() && !c.IsPaused() {
				// Do not autostart containers which
				// has endpoints in a swarm scope
				// network yet since the cluster is
				// not initialized yet. We will start
				// it after the cluster is
				// initialized.
				if daemon.configStore.AutoRestart && c.ShouldRestart() && !c.NetworkSettings.HasSwarmEndpoint {
					mapLock.Lock()
					restartContainers[c] = make(chan struct{})
					mapLock.Unlock()
				} else if c.HostConfig != nil && c.HostConfig.AutoRemove {
					mapLock.Lock()
					removeContainers[c.ID] = c
					mapLock.Unlock()
				}
			}

			if c.RemovalInProgress {
				// We probably crashed in the middle of a removal, reset
				// the flag.
				//
				// We DO NOT remove the container here as we do not
				// know if the user had requested for either the
				// associated volumes, network links or both to also
				// be removed. So we put the container in the "dead"
				// state and leave further processing up to them.
				logrus.Debugf("Resetting RemovalInProgress flag from %v", c.ID)
				c.ResetRemovalInProgress()
				c.SetDead()
				c.ToDisk()
			}
		}(c)
	}
	wg.Wait()
	daemon.netController, err = daemon.initNetworkController(daemon.configStore, activeSandboxes)
	if err != nil {
		return fmt.Errorf("Error initializing network controller: %v", err)
	}

	// Now that all the containers are registered, register the links
	for _, c := range containers {
		if err := daemon.registerLinks(c, c.HostConfig); err != nil {
			logrus.Errorf("failed to register link for container %s: %v", c.ID, err)
		}
	}

	group := sync.WaitGroup{}
	for c, notifier := range restartContainers {
		group.Add(1)

		go func(c *container.Container, chNotify chan struct{}) {
			defer group.Done()

			logrus.Debugf("Starting container %s", c.ID)

			// ignore errors here as this is a best effort to wait for children to be
			//   running before we try to start the container
			children := daemon.children(c)
			timeout := time.After(5 * time.Second)
			for _, child := range children {
				if notifier, exists := restartContainers[child]; exists {
					select {
					case <-notifier:
					case <-timeout:
					}
				}
			}

			// Make sure networks are available before starting
			daemon.waitForNetworks(c)
			if err := daemon.containerStart(c, "", "", true); err != nil {
				logrus.Errorf("Failed to start container %s: %s", c.ID, err)
			}
			close(chNotify)
		}(c, notifier)

	}
	group.Wait()

	removeGroup := sync.WaitGroup{}
	for id := range removeContainers {
		removeGroup.Add(1)
		go func(cid string) {
			if err := daemon.ContainerRm(cid, &types.ContainerRmConfig{ForceRemove: true, RemoveVolume: true}); err != nil {
				logrus.Errorf("Failed to remove container %s: %s", cid, err)
			}
			removeGroup.Done()
		}(id)
	}
	removeGroup.Wait()

	// any containers that were started above would already have had this done,
	// however we need to now prepare the mountpoints for the rest of the containers as well.
	// This shouldn't cause any issue running on the containers that already had this run.
	// This must be run after any containers with a restart policy so that containerized plugins
	// can have a chance to be running before we try to initialize them.
	for _, c := range containers {
		// if the container has restart policy, do not
		// prepare the mountpoints since it has been done on restarting.
		// This is to speed up the daemon start when a restart container
		// has a volume and the volume driver is not available.
		if _, ok := restartContainers[c]; ok {
			continue
		} else if _, ok := removeContainers[c.ID]; ok {
			// container is automatically removed, skip it.
			continue
		}

		group.Add(1)
		go func(c *container.Container) {
			defer group.Done()
			if err := daemon.prepareMountPoints(c); err != nil {
				logrus.Error(err)
			}
		}(c)
	}

	group.Wait()

	logrus.Info("Loading containers: done.")

	return nil
}

// RestartSwarmContainers restarts any autostart container which has a
// swarm endpoint.
func (daemon *Daemon) RestartSwarmContainers() {
	group := sync.WaitGroup{}
	for _, c := range daemon.List() {
		if !c.IsRunning() && !c.IsPaused() {
			// Autostart all the containers which has a
			// swarm endpoint now that the cluster is
			// initialized.
			if daemon.configStore.AutoRestart && c.ShouldRestart() && c.NetworkSettings.HasSwarmEndpoint {
				group.Add(1)
				go func(c *container.Container) {
					defer group.Done()
					if err := daemon.containerStart(c, "", "", true); err != nil {
						logrus.Error(err)
					}
				}(c)
			}
		}

	}
	group.Wait()
}

// waitForNetworks is used during daemon initialization when starting up containers
// It ensures that all of a container's networks are available before the daemon tries to start the container.
// In practice it just makes sure the discovery service is available for containers which use a network that require discovery.
func (daemon *Daemon) waitForNetworks(c *container.Container) {
	if daemon.discoveryWatcher == nil {
		return
	}
	// Make sure if the container has a network that requires discovery that the discovery service is available before starting
	for netName := range c.NetworkSettings.Networks {
		// If we get `ErrNoSuchNetwork` here, we can assume that it is due to discovery not being ready
		// Most likely this is because the K/V store used for discovery is in a container and needs to be started
		if _, err := daemon.netController.NetworkByName(netName); err != nil {
			if _, ok := err.(libnetwork.ErrNoSuchNetwork); !ok {
				continue
			}
			// use a longish timeout here due to some slowdowns in libnetwork if the k/v store is on anything other than --net=host
			// FIXME: why is this slow???
			logrus.Debugf("Container %s waiting for network to be ready", c.Name)
			select {
			case <-daemon.discoveryWatcher.ReadyCh():
			case <-time.After(60 * time.Second):
			}
			return
		}
	}
}

func (daemon *Daemon) children(c *container.Container) map[string]*container.Container {
	return daemon.linkIndex.children(c)
}

// parents returns the names of the parent containers of the container
// with the given name.
func (daemon *Daemon) parents(c *container.Container) map[string]*container.Container {
	return daemon.linkIndex.parents(c)
}

func (daemon *Daemon) registerLink(parent, child *container.Container, alias string) error {
	fullName := path.Join(parent.Name, alias)
	if err := daemon.nameIndex.Reserve(fullName, child.ID); err != nil {
		if err == registrar.ErrNameReserved {
			logrus.Warnf("error registering link for %s, to %s, as alias %s, ignoring: %v", parent.ID, child.ID, alias, err)
			return nil
		}
		return err
	}
	daemon.linkIndex.link(parent, child, fullName)
	return nil
}

// DaemonJoinsCluster informs the daemon has joined the cluster and provides
// the handler to query the cluster component
func (daemon *Daemon) DaemonJoinsCluster(clusterProvider cluster.Provider) {
	daemon.setClusterProvider(clusterProvider)
}

// DaemonLeavesCluster informs the daemon has left the cluster
func (daemon *Daemon) DaemonLeavesCluster() {
	// Daemon is in charge of removing the attachable networks with
	// connected containers when the node leaves the swarm
	daemon.clearAttachableNetworks()
	// We no longer need the cluster provider, stop it now so that
	// the network agent will stop listening to cluster events.
	daemon.setClusterProvider(nil)
	// Wait for the networking cluster agent to stop
	daemon.netController.AgentStopWait()
	// Daemon is in charge of removing the ingress network when the
	// node leaves the swarm. Wait for job to be done or timeout.
	// This is called also on graceful daemon shutdown. We need to
	// wait, because the ingress release has to happen before the
	// network controller is stopped.
	if done, err := daemon.ReleaseIngress(); err == nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			logrus.Warnf("timeout while waiting for ingress network removal")
		}
	} else {
		logrus.Warnf("failed to initiate ingress network removal: %v", err)
	}
}

// setClusterProvider sets a component for querying the current cluster state.
func (daemon *Daemon) setClusterProvider(clusterProvider cluster.Provider) {
	daemon.clusterProvider = clusterProvider
	daemon.netController.SetClusterProvider(clusterProvider)
}

// IsSwarmCompatible verifies if the current daemon
// configuration is compatible with the swarm mode
func (daemon *Daemon) IsSwarmCompatible() error {
	if daemon.configStore == nil {
		return nil
	}
	return daemon.configStore.IsSwarmCompatible()
}

// NewDaemon sets up everything for the daemon to be able to service
// requests from the webserver.
func NewDaemon(config *config.Config, registryService registry.Service, containerdRemote libcontainerd.Remote, pluginStore *plugin.Store) (daemon *Daemon, err error) {
	setDefaultMtu(config)

	// Ensure that we have a correct root key limit for launching containers.
	if err := ModifyRootKeyLimit(); err != nil {
		logrus.Warnf("unable to modify root key limit, number of containers could be limited by this quota: %v", err)
	}

	// Ensure we have compatible and valid configuration options
	if err := verifyDaemonSettings(config); err != nil {
		return nil, err
	}

	// Do we have a disabled network?
	config.DisableBridge = isBridgeNetworkDisabled(config)

	// Verify the platform is supported as a daemon
	if !platformSupported {
		return nil, errSystemNotSupported
	}

	// Validate platform-specific requirements
	if err := checkSystem(); err != nil {
		return nil, err
	}

	uidMaps, gidMaps, err := setupRemappedRoot(config)
	if err != nil {
		return nil, err
	}
	rootUID, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return nil, err
	}

	if err := setupDaemonProcess(config); err != nil {
		return nil, err
	}

	// set up the tmpDir to use a canonical path
	tmp, err := prepareTempDir(config.Root, rootUID, rootGID)
	if err != nil {
		return nil, fmt.Errorf("Unable to get the TempDir under %s: %s", config.Root, err)
	}
	realTmp, err := fileutils.ReadSymlinkedDirectory(tmp)
	if err != nil {
		return nil, fmt.Errorf("Unable to get the full path to the TempDir (%s): %s", tmp, err)
	}
	os.Setenv("TMPDIR", realTmp)

	d := &Daemon{configStore: config}
	// Ensure the daemon is properly shutdown if there is a failure during
	// initialization
	defer func() {
		if err != nil {
			if err := d.Shutdown(); err != nil {
				logrus.Error(err)
			}
		}
	}()

	// set up SIGUSR1 handler on Unix-like systems, or a Win32 global event
	// on Windows to dump Go routine stacks
	stackDumpDir := config.Root
	if execRoot := config.GetExecRoot(); execRoot != "" {
		stackDumpDir = execRoot
	}
	d.setupDumpStackTrap(stackDumpDir)

	if err := d.setupSeccompProfile(); err != nil {
		return nil, err
	}

	// Set the default isolation mode (only applicable on Windows)
	if err := d.setDefaultIsolation(); err != nil {
		return nil, fmt.Errorf("error setting default isolation mode: %v", err)
	}

	logrus.Debugf("Using default logging driver %s", config.LogConfig.Type)

	if err := configureMaxThreads(config); err != nil {
		logrus.Warnf("Failed to configure golang's threads limit: %v", err)
	}

	if err := ensureDefaultAppArmorProfile(); err != nil {
		logrus.Errorf(err.Error())
	}

	daemonRepo := filepath.Join(config.Root, "containers")
	if err := idtools.MkdirAllAs(daemonRepo, 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return nil, err
	}

	if runtime.GOOS == "windows" {
		if err := system.MkdirAll(filepath.Join(config.Root, "credentialspecs"), 0); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}

	driverName := os.Getenv("DOCKER_DRIVER")
	if driverName == "" {
		driverName = config.GraphDriver
	}

	d.RegistryService = registryService
	d.PluginStore = pluginStore

	// Plugin system initialization should happen before restore. Do not change order.
	d.pluginManager, err = plugin.NewManager(plugin.ManagerConfig{
		Root:               filepath.Join(config.Root, "plugins"),
		ExecRoot:           getPluginExecRoot(config.Root),
		Store:              d.PluginStore,
		Executor:           containerdRemote,
		RegistryService:    registryService,
		LiveRestoreEnabled: config.LiveRestoreEnabled,
		LogPluginEvent:     d.LogPluginEvent, // todo: make private
		AuthzMiddleware:    config.AuthzMiddleware,
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create plugin manager")
	}

	d.layerStore, err = layer.NewStoreFromOptions(layer.StoreOptions{
		StorePath:                 config.Root,
		MetadataStorePathTemplate: filepath.Join(config.Root, "image", "%s", "layerdb"),
		GraphDriver:               driverName,
		GraphDriverOptions:        config.GraphOptions,
		UIDMaps:                   uidMaps,
		GIDMaps:                   gidMaps,
		PluginGetter:              d.PluginStore,
		ExperimentalEnabled:       config.Experimental,
	})
	if err != nil {
		return nil, err
	}

	graphDriver := d.layerStore.DriverName()
	imageRoot := filepath.Join(config.Root, "image", graphDriver)

	// Configure and validate the kernels security support
	if err := configureKernelSecuritySupport(config, graphDriver); err != nil {
		return nil, err
	}

	logrus.Debugf("Max Concurrent Downloads: %d", *config.MaxConcurrentDownloads)
	d.downloadManager = xfer.NewLayerDownloadManager(d.layerStore, *config.MaxConcurrentDownloads)
	logrus.Debugf("Max Concurrent Uploads: %d", *config.MaxConcurrentUploads)
	d.uploadManager = xfer.NewLayerUploadManager(*config.MaxConcurrentUploads)

	ifs, err := image.NewFSStoreBackend(filepath.Join(imageRoot, "imagedb"))
	if err != nil {
		return nil, err
	}

	d.imageStore, err = image.NewImageStore(ifs, d.layerStore)
	if err != nil {
		return nil, err
	}

	// Configure the volumes driver
	volStore, err := d.configureVolumes(rootUID, rootGID)
	if err != nil {
		return nil, err
	}

	trustKey, err := api.LoadOrCreateTrustKey(config.TrustKeyPath)
	if err != nil {
		return nil, err
	}

	trustDir := filepath.Join(config.Root, "trust")

	if err := system.MkdirAll(trustDir, 0700); err != nil {
		return nil, err
	}

	distributionMetadataStore, err := dmetadata.NewFSMetadataStore(filepath.Join(imageRoot, "distribution"))
	if err != nil {
		return nil, err
	}

	eventsService := events.New()

	referenceStore, err := refstore.NewReferenceStore(filepath.Join(imageRoot, "repositories.json"))
	if err != nil {
		return nil, fmt.Errorf("Couldn't create Tag store repositories: %s", err)
	}

	migrationStart := time.Now()
	if err := v1.Migrate(config.Root, graphDriver, d.layerStore, d.imageStore, referenceStore, distributionMetadataStore); err != nil {
		logrus.Errorf("Graph migration failed: %q. Your old graph data was found to be too inconsistent for upgrading to content-addressable storage. Some of the old data was probably not upgraded. We recommend starting over with a clean storage directory if possible.", err)
	}
	logrus.Infof("Graph migration to content-addressability took %.2f seconds", time.Since(migrationStart).Seconds())

	// Discovery is only enabled when the daemon is launched with an address to advertise.  When
	// initialized, the daemon is registered and we can store the discovery backend as it's read-only
	if err := d.initDiscovery(config); err != nil {
		return nil, err
	}

	sysInfo := sysinfo.New(false)
	// Check if Devices cgroup is mounted, it is hard requirement for container security,
	// on Linux.
	if runtime.GOOS == "linux" && !sysInfo.CgroupDevicesEnabled {
		return nil, errors.New("Devices cgroup isn't mounted")
	}

	d.ID = trustKey.PublicKey().KeyID()
	d.repository = daemonRepo
	d.containers = container.NewMemoryStore()
	d.execCommands = exec.NewStore()
	d.referenceStore = referenceStore
	d.distributionMetadataStore = distributionMetadataStore
	d.trustKey = trustKey
	d.idIndex = truncindex.NewTruncIndex([]string{})
	d.statsCollector = d.newStatsCollector(1 * time.Second)
	d.defaultLogConfig = containertypes.LogConfig{
		Type:   config.LogConfig.Type,
		Config: config.LogConfig.Config,
	}
	d.EventsService = eventsService
	d.volumes = volStore
	d.root = config.Root
	d.uidMaps = uidMaps
	d.gidMaps = gidMaps
	d.seccompEnabled = sysInfo.Seccomp
	d.apparmorEnabled = sysInfo.AppArmor

	d.nameIndex = registrar.NewRegistrar()
	d.linkIndex = newLinkIndex()
	d.containerdRemote = containerdRemote

	go d.execCommandGC()

	d.containerd, err = containerdRemote.Client(d)
	if err != nil {
		return nil, err
	}

	if err := d.restore(); err != nil {
		return nil, err
	}

	// FIXME: this method never returns an error
	info, _ := d.SystemInfo()

	engineVersion.WithValues(
		dockerversion.Version,
		dockerversion.GitCommit,
		info.Architecture,
		info.Driver,
		info.KernelVersion,
		info.OperatingSystem,
	).Set(1)
	engineCpus.Set(float64(info.NCPU))
	engineMemory.Set(float64(info.MemTotal))

	return d, nil
}

func (daemon *Daemon) shutdownContainer(c *container.Container) error {
	stopTimeout := c.StopTimeout()
	// TODO(windows): Handle docker restart with paused containers
	if c.IsPaused() {
		// To terminate a process in freezer cgroup, we should send
		// SIGTERM to this process then unfreeze it, and the process will
		// force to terminate immediately.
		logrus.Debugf("Found container %s is paused, sending SIGTERM before unpausing it", c.ID)
		sig, ok := signal.SignalMap["TERM"]
		if !ok {
			return errors.New("System does not support SIGTERM")
		}
		if err := daemon.kill(c, int(sig)); err != nil {
			return fmt.Errorf("sending SIGTERM to container %s with error: %v", c.ID, err)
		}
		if err := daemon.containerUnpause(c); err != nil {
			return fmt.Errorf("Failed to unpause container %s with error: %v", c.ID, err)
		}
		if _, err := c.WaitStop(time.Duration(stopTimeout) * time.Second); err != nil {
			logrus.Debugf("container %s failed to exit in %d second of SIGTERM, sending SIGKILL to force", c.ID, stopTimeout)
			sig, ok := signal.SignalMap["KILL"]
			if !ok {
				return errors.New("System does not support SIGKILL")
			}
			if err := daemon.kill(c, int(sig)); err != nil {
				logrus.Errorf("Failed to SIGKILL container %s", c.ID)
			}
			c.WaitStop(-1 * time.Second)
			return err
		}
	}
	// If container failed to exit in stopTimeout seconds of SIGTERM, then using the force
	if err := daemon.containerStop(c, stopTimeout); err != nil {
		return fmt.Errorf("Failed to stop container %s with error: %v", c.ID, err)
	}

	c.WaitStop(-1 * time.Second)
	return nil
}

// ShutdownTimeout returns the shutdown timeout based on the max stopTimeout of the containers,
// and is limited by daemon's ShutdownTimeout.
func (daemon *Daemon) ShutdownTimeout() int {
	// By default we use daemon's ShutdownTimeout.
	shutdownTimeout := daemon.configStore.ShutdownTimeout

	graceTimeout := 5
	if daemon.containers != nil {
		for _, c := range daemon.containers.List() {
			if shutdownTimeout >= 0 {
				stopTimeout := c.StopTimeout()
				if stopTimeout < 0 {
					shutdownTimeout = -1
				} else {
					if stopTimeout+graceTimeout > shutdownTimeout {
						shutdownTimeout = stopTimeout + graceTimeout
					}
				}
			}
		}
	}
	return shutdownTimeout
}

// Shutdown stops the daemon.
func (daemon *Daemon) Shutdown() error {
	daemon.shutdown = true
	// Keep mounts and networking running on daemon shutdown if
	// we are to keep containers running and restore them.

	if daemon.configStore.LiveRestoreEnabled && daemon.containers != nil {
		// check if there are any running containers, if none we should do some cleanup
		if ls, err := daemon.Containers(&types.ContainerListOptions{}); len(ls) != 0 || err != nil {
			return nil
		}
	}

	if daemon.containers != nil {
		logrus.Debugf("start clean shutdown of all containers with a %d seconds timeout...", daemon.configStore.ShutdownTimeout)
		daemon.containers.ApplyAll(func(c *container.Container) {
			if !c.IsRunning() {
				return
			}
			logrus.Debugf("stopping %s", c.ID)
			if err := daemon.shutdownContainer(c); err != nil {
				logrus.Errorf("Stop container error: %v", err)
				return
			}
			if mountid, err := daemon.layerStore.GetMountID(c.ID); err == nil {
				daemon.cleanupMountsByID(mountid)
			}
			logrus.Debugf("container stopped %s", c.ID)
		})
	}

	if daemon.volumes != nil {
		if err := daemon.volumes.Shutdown(); err != nil {
			logrus.Errorf("Error shutting down volume store: %v", err)
		}
	}

	if daemon.layerStore != nil {
		if err := daemon.layerStore.Cleanup(); err != nil {
			logrus.Errorf("Error during layer Store.Cleanup(): %v", err)
		}
	}

	// If we are part of a cluster, clean up cluster's stuff
	if daemon.clusterProvider != nil {
		logrus.Debugf("start clean shutdown of cluster resources...")
		daemon.DaemonLeavesCluster()
	}

	// Shutdown plugins after containers and layerstore. Don't change the order.
	daemon.pluginShutdown()

	// trigger libnetwork Stop only if it's initialized
	if daemon.netController != nil {
		daemon.netController.Stop()
	}

	if err := daemon.cleanupMounts(); err != nil {
		return err
	}

	return nil
}

// Mount sets container.BaseFS
// (is it not set coming in? why is it unset?)
func (daemon *Daemon) Mount(container *container.Container) error {
	dir, err := container.RWLayer.Mount(container.GetMountLabel())
	if err != nil {
		return err
	}
	logrus.Debugf("container mounted via layerStore: %v", dir)

	if container.BaseFS != dir {
		// The mount path reported by the graph driver should always be trusted on Windows, since the
		// volume path for a given mounted layer may change over time.  This should only be an error
		// on non-Windows operating systems.
		if container.BaseFS != "" && runtime.GOOS != "windows" {
			daemon.Unmount(container)
			return fmt.Errorf("Error: driver %s is returning inconsistent paths for container %s ('%s' then '%s')",
				daemon.GraphDriverName(), container.ID, container.BaseFS, dir)
		}
	}
	container.BaseFS = dir // TODO: combine these fields
	return nil
}

// Unmount unsets the container base filesystem
func (daemon *Daemon) Unmount(container *container.Container) error {
	if err := container.RWLayer.Unmount(); err != nil {
		logrus.Errorf("Error unmounting container %s: %s", container.ID, err)
		return err
	}

	return nil
}

// Subnets return the IPv4 and IPv6 subnets of networks that are manager by Docker.
func (daemon *Daemon) Subnets() ([]net.IPNet, []net.IPNet) {
	var v4Subnets []net.IPNet
	var v6Subnets []net.IPNet

	managedNetworks := daemon.netController.Networks()

	for _, managedNetwork := range managedNetworks {
		v4infos, v6infos := managedNetwork.Info().IpamInfo()
		for _, info := range v4infos {
			if info.IPAMData.Pool != nil {
				v4Subnets = append(v4Subnets, *info.IPAMData.Pool)
			}
		}
		for _, info := range v6infos {
			if info.IPAMData.Pool != nil {
				v6Subnets = append(v6Subnets, *info.IPAMData.Pool)
			}
		}
	}

	return v4Subnets, v6Subnets
}

// GraphDriverName returns the name of the graph driver used by the layer.Store
func (daemon *Daemon) GraphDriverName() string {
	return daemon.layerStore.DriverName()
}

// GetUIDGIDMaps returns the current daemon's user namespace settings
// for the full uid and gid maps which will be applied to containers
// started in this instance.
func (daemon *Daemon) GetUIDGIDMaps() ([]idtools.IDMap, []idtools.IDMap) {
	return daemon.uidMaps, daemon.gidMaps
}

// GetRemappedUIDGID returns the current daemon's uid and gid values
// if user namespaces are in use for this daemon instance.  If not
// this function will return "real" root values of 0, 0.
func (daemon *Daemon) GetRemappedUIDGID() (int, int) {
	uid, gid, _ := idtools.GetRootUIDGID(daemon.uidMaps, daemon.gidMaps)
	return uid, gid
}

// prepareTempDir prepares and returns the default directory to use
// for temporary files.
// If it doesn't exist, it is created. If it exists, its content is removed.
func prepareTempDir(rootDir string, rootUID, rootGID int) (string, error) {
	var tmpDir string
	if tmpDir = os.Getenv("DOCKER_TMPDIR"); tmpDir == "" {
		tmpDir = filepath.Join(rootDir, "tmp")
		newName := tmpDir + "-old"
		if err := os.Rename(tmpDir, newName); err != nil {
			go func() {
				if err := os.RemoveAll(newName); err != nil {
					logrus.Warnf("failed to delete old tmp directory: %s", newName)
				}
			}()
		} else {
			logrus.Warnf("failed to rename %s for background deletion: %s. Deleting synchronously", tmpDir, err)
			if err := os.RemoveAll(tmpDir); err != nil {
				logrus.Warnf("failed to delete old tmp directory: %s", tmpDir)
			}
		}
	}
	// We don't remove the content of tmpdir if it's not the default,
	// it may hold things that do not belong to us.
	return tmpDir, idtools.MkdirAllAs(tmpDir, 0700, rootUID, rootGID)
}

func (daemon *Daemon) setupInitLayer(initPath string) error {
	rootUID, rootGID := daemon.GetRemappedUIDGID()
	return initlayer.Setup(initPath, rootUID, rootGID)
}

func setDefaultMtu(conf *config.Config) {
	// do nothing if the config does not have the default 0 value.
	if conf.Mtu != 0 {
		return
	}
	conf.Mtu = config.DefaultNetworkMtu
}

func (daemon *Daemon) configureVolumes(rootUID, rootGID int) (*store.VolumeStore, error) {
	volumesDriver, err := local.New(daemon.configStore.Root, rootUID, rootGID)
	if err != nil {
		return nil, err
	}

	volumedrivers.RegisterPluginGetter(daemon.PluginStore)

	if !volumedrivers.Register(volumesDriver, volumesDriver.Name()) {
		return nil, errors.New("local volume driver could not be registered")
	}
	return store.New(daemon.configStore.Root)
}

// IsShuttingDown tells whether the daemon is shutting down or not
func (daemon *Daemon) IsShuttingDown() bool {
	return daemon.shutdown
}

// initDiscovery initializes the discovery watcher for this daemon.
func (daemon *Daemon) initDiscovery(conf *config.Config) error {
	advertise, err := config.ParseClusterAdvertiseSettings(conf.ClusterStore, conf.ClusterAdvertise)
	if err != nil {
		if err == discovery.ErrDiscoveryDisabled {
			return nil
		}
		return err
	}

	conf.ClusterAdvertise = advertise
	discoveryWatcher, err := discovery.Init(conf.ClusterStore, conf.ClusterAdvertise, conf.ClusterOpts)
	if err != nil {
		return fmt.Errorf("discovery initialization failed (%v)", err)
	}

	daemon.discoveryWatcher = discoveryWatcher
	return nil
}

func isBridgeNetworkDisabled(conf *config.Config) bool {
	return conf.BridgeConfig.Iface == config.DisableNetworkBridge
}

func (daemon *Daemon) networkOptions(dconfig *config.Config, pg plugingetter.PluginGetter, activeSandboxes map[string]interface{}) ([]nwconfig.Option, error) {
	options := []nwconfig.Option{}
	if dconfig == nil {
		return options, nil
	}

	options = append(options, nwconfig.OptionExperimental(dconfig.Experimental))
	options = append(options, nwconfig.OptionDataDir(dconfig.Root))
	options = append(options, nwconfig.OptionExecRoot(dconfig.GetExecRoot()))

	dd := runconfig.DefaultDaemonNetworkMode()
	dn := runconfig.DefaultDaemonNetworkMode().NetworkName()
	options = append(options, nwconfig.OptionDefaultDriver(string(dd)))
	options = append(options, nwconfig.OptionDefaultNetwork(dn))

	if strings.TrimSpace(dconfig.ClusterStore) != "" {
		kv := strings.Split(dconfig.ClusterStore, "://")
		if len(kv) != 2 {
			return nil, errors.New("kv store daemon config must be of the form KV-PROVIDER://KV-URL")
		}
		options = append(options, nwconfig.OptionKVProvider(kv[0]))
		options = append(options, nwconfig.OptionKVProviderURL(kv[1]))
	}
	if len(dconfig.ClusterOpts) > 0 {
		options = append(options, nwconfig.OptionKVOpts(dconfig.ClusterOpts))
	}

	if daemon.discoveryWatcher != nil {
		options = append(options, nwconfig.OptionDiscoveryWatcher(daemon.discoveryWatcher))
	}

	if dconfig.ClusterAdvertise != "" {
		options = append(options, nwconfig.OptionDiscoveryAddress(dconfig.ClusterAdvertise))
	}

	options = append(options, nwconfig.OptionLabels(dconfig.Labels))
	options = append(options, driverOptions(dconfig)...)

	if daemon.configStore != nil && daemon.configStore.LiveRestoreEnabled && len(activeSandboxes) != 0 {
		options = append(options, nwconfig.OptionActiveSandboxes(activeSandboxes))
	}

	if pg != nil {
		options = append(options, nwconfig.OptionPluginGetter(pg))
	}

	return options, nil
}

func copyBlkioEntry(entries []*containerd.BlkioStatsEntry) []types.BlkioStatEntry {
	out := make([]types.BlkioStatEntry, len(entries))
	for i, re := range entries {
		out[i] = types.BlkioStatEntry{
			Major: re.Major,
			Minor: re.Minor,
			Op:    re.Op,
			Value: re.Value,
		}
	}
	return out
}

// GetCluster returns the cluster
func (daemon *Daemon) GetCluster() Cluster {
	return daemon.cluster
}

// SetCluster sets the cluster
func (daemon *Daemon) SetCluster(cluster Cluster) {
	daemon.cluster = cluster
}

func (daemon *Daemon) pluginShutdown() {
	manager := daemon.pluginManager
	// Check for a valid manager object. In error conditions, daemon init can fail
	// and shutdown called, before plugin manager is initialized.
	if manager != nil {
		manager.Shutdown()
	}
}

// PluginManager returns current pluginManager associated with the daemon
func (daemon *Daemon) PluginManager() *plugin.Manager { // set up before daemon to avoid this method
	return daemon.pluginManager
}

// PluginGetter returns current pluginStore associated with the daemon
func (daemon *Daemon) PluginGetter() *plugin.Store {
	return daemon.PluginStore
}

// CreateDaemonRoot creates the root for the daemon
func CreateDaemonRoot(config *config.Config) error {
	// get the canonical path to the Docker root directory
	var realRoot string
	if _, err := os.Stat(config.Root); err != nil && os.IsNotExist(err) {
		realRoot = config.Root
	} else {
		realRoot, err = fileutils.ReadSymlinkedDirectory(config.Root)
		if err != nil {
			return fmt.Errorf("Unable to get the full path to root (%s): %s", config.Root, err)
		}
	}

	uidMaps, gidMaps, err := setupRemappedRoot(config)
	if err != nil {
		return err
	}
	rootUID, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return err
	}

	if err := setupDaemonRoot(config, realRoot, rootUID, rootGID); err != nil {
		return err
	}

	return nil
}
