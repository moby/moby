// Package daemon exposes the functions that occur on the host server
// that the Docker daemon is running.
//
// In implementing the various functions of the daemon, there is often
// a method-specific struct for configuring the runtime behavior.
package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/pkg/dialer"
	"github.com/containerd/containerd/pkg/userns"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	ctrd "github.com/docker/docker/daemon/containerd"
	"github.com/docker/docker/daemon/events"
	_ "github.com/docker/docker/daemon/graphdriver/register" // register graph drivers
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/daemon/stats"
	dmetadata "github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	libcontainerdtypes "github.com/docker/docker/libcontainerd/types"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/libnetwork/cluster"
	nwconfig "github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/plugin"
	pluginexec "github.com/docker/docker/plugin/executor/containerd"
	refstore "github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	volumesservice "github.com/docker/docker/volume/service"
	"github.com/moby/buildkit/util/resolver"
	resolverconfig "github.com/moby/buildkit/util/resolver/config"
	"github.com/moby/locker"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
	"golang.org/x/sync/semaphore"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// Daemon holds information about the Docker daemon.
type Daemon struct {
	id                    string
	repository            string
	containers            container.Store
	containersReplica     *container.ViewDB
	execCommands          *container.ExecStore
	imageService          ImageService
	configStore           *config.Config
	statsCollector        *stats.Collector
	defaultLogConfig      containertypes.LogConfig
	registryService       registry.Service
	EventsService         *events.Events
	netController         libnetwork.NetworkController
	volumes               *volumesservice.VolumesService
	root                  string
	sysInfoOnce           sync.Once
	sysInfo               *sysinfo.SysInfo
	shutdown              bool
	idMapping             idtools.IdentityMapping
	PluginStore           *plugin.Store // TODO: remove
	pluginManager         *plugin.Manager
	linkIndex             *linkIndex
	containerdCli         *containerd.Client
	containerd            libcontainerdtypes.Client
	defaultIsolation      containertypes.Isolation // Default isolation mode on Windows
	clusterProvider       cluster.Provider
	cluster               Cluster
	genericResources      []swarm.GenericResource
	metricsPluginListener net.Listener
	ReferenceStore        refstore.Store

	machineMemory uint64

	seccompProfile     []byte
	seccompProfilePath string

	usage singleflight.Group

	pruneRunning int32
	hosts        map[string]bool // hosts stores the addresses the daemon is listening on
	startupDone  chan struct{}

	attachmentStore       network.AttachmentStore
	attachableNetworkLock *locker.Locker

	// This is used for Windows which doesn't currently support running on containerd
	// It stores metadata for the content store (used for manifest caching)
	// This needs to be closed on daemon exit
	mdDB *bbolt.DB
}

// StoreHosts stores the addresses the daemon is listening on
func (daemon *Daemon) StoreHosts(hosts []string) {
	if daemon.hosts == nil {
		daemon.hosts = make(map[string]bool)
	}
	for _, h := range hosts {
		daemon.hosts[h] = true
	}
}

// HasExperimental returns whether the experimental features of the daemon are enabled or not
func (daemon *Daemon) HasExperimental() bool {
	return daemon.configStore != nil && daemon.configStore.Experimental
}

// Features returns the features map from configStore
func (daemon *Daemon) Features() *map[string]bool {
	return &daemon.configStore.Features
}

// UsesSnapshotter returns true if feature flag to use containerd snapshotter is enabled
func (daemon *Daemon) UsesSnapshotter() bool {
	if daemon.configStore.Features != nil {
		if b, ok := daemon.configStore.Features["containerd-snapshotter"]; ok {
			return b
		}
	}
	return false
}

// RegistryHosts returns registry configuration in containerd resolvers format
func (daemon *Daemon) RegistryHosts() docker.RegistryHosts {
	var (
		registryKey = "docker.io"
		mirrors     = make([]string, len(daemon.configStore.Mirrors))
		m           = map[string]resolverconfig.RegistryConfig{}
	)
	// must trim "https://" or "http://" prefix
	for i, v := range daemon.configStore.Mirrors {
		if uri, err := url.Parse(v); err == nil {
			v = uri.Host
		}
		mirrors[i] = v
	}
	// set mirrors for default registry
	m[registryKey] = resolverconfig.RegistryConfig{Mirrors: mirrors}

	for _, v := range daemon.configStore.InsecureRegistries {
		u, err := url.Parse(v)
		c := resolverconfig.RegistryConfig{}
		if err == nil {
			v = u.Host
			t := true
			if u.Scheme == "http" {
				c.PlainHTTP = &t
			} else {
				c.Insecure = &t
			}
		}
		m[v] = c
	}

	for k, v := range m {
		v.TLSConfigDir = []string{registry.HostCertsDir(k)}
		m[k] = v
	}

	certsDir := registry.CertsDir()
	if fis, err := os.ReadDir(certsDir); err == nil {
		for _, fi := range fis {
			if _, ok := m[fi.Name()]; !ok {
				m[fi.Name()] = resolverconfig.RegistryConfig{
					TLSConfigDir: []string{filepath.Join(certsDir, fi.Name())},
				}
			}
		}
	}

	return resolver.NewRegistryConfig(m)
}

func (daemon *Daemon) restore(ctx context.Context) error {
	var mapLock sync.Mutex
	containers := make(map[string]*container.Container)

	logrus.Info("Loading containers: start.")

	dir, err := os.ReadDir(daemon.repository)
	if err != nil {
		return err
	}

	// parallelLimit is the maximum number of parallel startup jobs that we
	// allow (this is the limited used for all startup semaphores). The multipler
	// (128) was chosen after some fairly significant benchmarking -- don't change
	// it unless you've tested it significantly (this value is adjusted if
	// RLIMIT_NOFILE is small to avoid EMFILE).
	parallelLimit := adjustParallelLimit(len(dir), 128*runtime.NumCPU())

	// Re-used for all parallel startup jobs.
	var group sync.WaitGroup
	sem := semaphore.NewWeighted(int64(parallelLimit))

	for _, v := range dir {
		group.Add(1)
		go func(id string) {
			defer group.Done()
			_ = sem.Acquire(context.Background(), 1)
			defer sem.Release(1)

			log := logrus.WithField("container", id)

			c, err := daemon.load(id)
			if err != nil {
				log.WithError(err).Error("failed to load container")
				return
			}
			if c.Driver != daemon.imageService.StorageDriver() {
				// Ignore the container if it wasn't created with the current storage-driver
				log.Debugf("not restoring container because it was created with another storage driver (%s)", c.Driver)
				return
			}
			rwlayer, err := daemon.imageService.GetLayerByID(c.ID)
			if err != nil {
				log.WithError(err).Error("failed to load container mount")
				return
			}
			c.RWLayer = rwlayer
			log.WithFields(logrus.Fields{
				"running": c.IsRunning(),
				"paused":  c.IsPaused(),
			}).Debug("loaded container")

			mapLock.Lock()
			containers[c.ID] = c
			mapLock.Unlock()
		}(v.Name())
	}
	group.Wait()

	removeContainers := make(map[string]*container.Container)
	restartContainers := make(map[*container.Container]chan struct{})
	activeSandboxes := make(map[string]interface{})

	for _, c := range containers {
		group.Add(1)
		go func(c *container.Container) {
			defer group.Done()
			_ = sem.Acquire(context.Background(), 1)
			defer sem.Release(1)

			log := logrus.WithField("container", c.ID)

			if err := daemon.registerName(c); err != nil {
				log.WithError(err).Errorf("failed to register container name: %s", c.Name)
				mapLock.Lock()
				delete(containers, c.ID)
				mapLock.Unlock()
				return
			}
			if err := daemon.Register(c); err != nil {
				log.WithError(err).Error("failed to register container")
				mapLock.Lock()
				delete(containers, c.ID)
				mapLock.Unlock()
				return
			}
		}(c)
	}
	group.Wait()

	for _, c := range containers {
		group.Add(1)
		go func(c *container.Container) {
			defer group.Done()
			_ = sem.Acquire(context.Background(), 1)
			defer sem.Release(1)

			log := logrus.WithField("container", c.ID)

			if err := daemon.checkpointAndSave(c); err != nil {
				log.WithError(err).Error("error saving backported mountspec to disk")
			}

			daemon.setStateCounter(c)

			logger := func(c *container.Container) *logrus.Entry {
				return log.WithFields(logrus.Fields{
					"running":    c.IsRunning(),
					"paused":     c.IsPaused(),
					"restarting": c.IsRestarting(),
				})
			}

			logger(c).Debug("restoring container")

			var es *containerd.ExitStatus

			if err := c.RestoreTask(context.Background(), daemon.containerd); err != nil && !errdefs.IsNotFound(err) {
				logger(c).WithError(err).Error("failed to restore container with containerd")
				return
			}

			alive := false
			status := containerd.Unknown
			if tsk, ok := c.Task(); ok {
				s, err := tsk.Status(context.Background())
				if err != nil {
					logger(c).WithError(err).Error("failed to get task status")
				} else {
					status = s.Status
					alive = status != containerd.Stopped
					if !alive {
						logger(c).Debug("cleaning up dead container process")
						es, err = tsk.Delete(context.Background())
						if err != nil && !errdefs.IsNotFound(err) {
							logger(c).WithError(err).Error("failed to delete task from containerd")
							return
						}
					} else if !daemon.configStore.LiveRestoreEnabled {
						logger(c).Debug("shutting down container considered alive by containerd")
						if err := daemon.shutdownContainer(c); err != nil && !errdefs.IsNotFound(err) {
							log.WithError(err).Error("error shutting down container")
							return
						}
						status = containerd.Stopped
						alive = false
						c.ResetRestartManager(false)
					}
				}
			}
			// If the containerd task for the container was not found, docker's view of the
			// container state will be updated accordingly via SetStopped further down.

			if c.IsRunning() || c.IsPaused() {
				logger(c).Debug("syncing container on disk state with real state")

				c.RestartManager().Cancel() // manually start containers because some need to wait for swarm networking

				switch {
				case c.IsPaused() && alive:
					logger(c).WithField("state", status).Info("restored container paused")
					switch status {
					case containerd.Paused, containerd.Pausing:
						// nothing to do
					case containerd.Unknown, containerd.Stopped, "":
						log.WithField("status", status).Error("unexpected status for paused container during restore")
					default:
						// running
						c.Lock()
						c.Paused = false
						daemon.setStateCounter(c)
						daemon.updateHealthMonitor(c)
						if err := c.CheckpointTo(daemon.containersReplica); err != nil {
							log.WithError(err).Error("failed to update paused container state")
						}
						c.Unlock()
					}
				case !c.IsPaused() && alive:
					logger(c).Debug("restoring healthcheck")
					c.Lock()
					daemon.updateHealthMonitor(c)
					c.Unlock()
				}

				if !alive {
					logger(c).Debug("setting stopped state")
					c.Lock()
					var ces container.ExitStatus
					if es != nil {
						ces.ExitCode = int(es.ExitCode())
						ces.ExitedAt = es.ExitTime()
					}
					c.SetStopped(&ces)
					daemon.Cleanup(c)
					if err := c.CheckpointTo(daemon.containersReplica); err != nil {
						log.WithError(err).Error("failed to update stopped container state")
					}
					c.Unlock()
					logger(c).Debug("set stopped state")
				}

				// we call Mount and then Unmount to get BaseFs of the container
				if err := daemon.Mount(c); err != nil {
					// The mount is unlikely to fail. However, in case mount fails
					// the container should be allowed to restore here. Some functionalities
					// (like docker exec -u user) might be missing but container is able to be
					// stopped/restarted/removed.
					// See #29365 for related information.
					// The error is only logged here.
					logger(c).WithError(err).Warn("failed to mount container to get BaseFs path")
				} else {
					if err := daemon.Unmount(c); err != nil {
						logger(c).WithError(err).Warn("failed to umount container to get BaseFs path")
					}
				}

				c.ResetRestartManager(false)
				if !c.HostConfig.NetworkMode.IsContainer() && c.IsRunning() {
					options, err := daemon.buildSandboxOptions(c)
					if err != nil {
						logger(c).WithError(err).Warn("failed to build sandbox option to restore container")
					}
					mapLock.Lock()
					activeSandboxes[c.NetworkSettings.SandboxID] = options
					mapLock.Unlock()
				}
			}

			// get list of containers we need to restart

			// Do not autostart containers which
			// has endpoints in a swarm scope
			// network yet since the cluster is
			// not initialized yet. We will start
			// it after the cluster is
			// initialized.
			if daemon.configStore.AutoRestart && c.ShouldRestart() && !c.NetworkSettings.HasSwarmEndpoint && c.HasBeenStartedBefore {
				mapLock.Lock()
				restartContainers[c] = make(chan struct{})
				mapLock.Unlock()
			} else if c.HostConfig != nil && c.HostConfig.AutoRemove {
				mapLock.Lock()
				removeContainers[c.ID] = c
				mapLock.Unlock()
			}

			c.Lock()
			if c.RemovalInProgress {
				// We probably crashed in the middle of a removal, reset
				// the flag.
				//
				// We DO NOT remove the container here as we do not
				// know if the user had requested for either the
				// associated volumes, network links or both to also
				// be removed. So we put the container in the "dead"
				// state and leave further processing up to them.
				c.RemovalInProgress = false
				c.Dead = true
				if err := c.CheckpointTo(daemon.containersReplica); err != nil {
					log.WithError(err).Error("failed to update RemovalInProgress container state")
				} else {
					log.Debugf("reset RemovalInProgress state for container")
				}
			}
			c.Unlock()
			logger(c).Debug("done restoring container")
		}(c)
	}
	group.Wait()

	// Initialize the network controller and configure network settings.
	//
	// Note that we cannot initialize the network controller earlier, as it
	// needs to know if there's active sandboxes (running containers).
	if err = daemon.initNetworkController(activeSandboxes); err != nil {
		return fmt.Errorf("Error initializing network controller: %v", err)
	}

	// Now that all the containers are registered, register the links
	for _, c := range containers {
		group.Add(1)
		go func(c *container.Container) {
			_ = sem.Acquire(context.Background(), 1)

			if err := daemon.registerLinks(c, c.HostConfig); err != nil {
				logrus.WithField("container", c.ID).WithError(err).Error("failed to register link for container")
			}

			sem.Release(1)
			group.Done()
		}(c)
	}
	group.Wait()

	for c, notifier := range restartContainers {
		group.Add(1)
		go func(c *container.Container, chNotify chan struct{}) {
			_ = sem.Acquire(context.Background(), 1)

			log := logrus.WithField("container", c.ID)

			log.Debug("starting container")

			// ignore errors here as this is a best effort to wait for children to be
			//   running before we try to start the container
			children := daemon.children(c)
			timeout := time.NewTimer(5 * time.Second)
			defer timeout.Stop()

			for _, child := range children {
				if notifier, exists := restartContainers[child]; exists {
					select {
					case <-notifier:
					case <-timeout.C:
					}
				}
			}

			if err := daemon.prepareMountPoints(c); err != nil {
				log.WithError(err).Error("failed to prepare mount points for container")
			}
			if err := daemon.containerStart(ctx, c, "", "", true); err != nil {
				log.WithError(err).Error("failed to start container")
			}
			close(chNotify)

			sem.Release(1)
			group.Done()
		}(c, notifier)
	}
	group.Wait()

	for id := range removeContainers {
		group.Add(1)
		go func(cid string) {
			_ = sem.Acquire(context.Background(), 1)

			if err := daemon.ContainerRm(cid, &types.ContainerRmConfig{ForceRemove: true, RemoveVolume: true}); err != nil {
				logrus.WithField("container", cid).WithError(err).Error("failed to remove container")
			}

			sem.Release(1)
			group.Done()
		}(id)
	}
	group.Wait()

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
			_ = sem.Acquire(context.Background(), 1)

			if err := daemon.prepareMountPoints(c); err != nil {
				logrus.WithField("container", c.ID).WithError(err).Error("failed to prepare mountpoints for container")
			}

			sem.Release(1)
			group.Done()
		}(c)
	}
	group.Wait()

	logrus.Info("Loading containers: done.")

	return nil
}

// RestartSwarmContainers restarts any autostart container which has a
// swarm endpoint.
func (daemon *Daemon) RestartSwarmContainers() {
	ctx := context.Background()

	// parallelLimit is the maximum number of parallel startup jobs that we
	// allow (this is the limited used for all startup semaphores). The multipler
	// (128) was chosen after some fairly significant benchmarking -- don't change
	// it unless you've tested it significantly (this value is adjusted if
	// RLIMIT_NOFILE is small to avoid EMFILE).
	parallelLimit := adjustParallelLimit(len(daemon.List()), 128*runtime.NumCPU())

	var group sync.WaitGroup
	sem := semaphore.NewWeighted(int64(parallelLimit))

	for _, c := range daemon.List() {
		if !c.IsRunning() && !c.IsPaused() {
			// Autostart all the containers which has a
			// swarm endpoint now that the cluster is
			// initialized.
			if daemon.configStore.AutoRestart && c.ShouldRestart() && c.NetworkSettings.HasSwarmEndpoint && c.HasBeenStartedBefore {
				group.Add(1)
				go func(c *container.Container) {
					if err := sem.Acquire(ctx, 1); err != nil {
						// ctx is done.
						group.Done()
						return
					}

					if err := daemon.containerStart(ctx, c, "", "", true); err != nil {
						logrus.WithField("container", c.ID).WithError(err).Error("failed to start swarm container")
					}

					sem.Release(1)
					group.Done()
				}(c)
			}
		}
	}
	group.Wait()
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
	if err := daemon.containersReplica.ReserveName(fullName, child.ID); err != nil {
		if err == container.ErrNameReserved {
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
		timeout := time.NewTimer(5 * time.Second)
		defer timeout.Stop()

		select {
		case <-done:
		case <-timeout.C:
			logrus.Warn("timeout while waiting for ingress network removal")
		}
	} else {
		logrus.Warnf("failed to initiate ingress network removal: %v", err)
	}

	daemon.attachmentStore.ClearAttachments()
}

// setClusterProvider sets a component for querying the current cluster state.
func (daemon *Daemon) setClusterProvider(clusterProvider cluster.Provider) {
	daemon.clusterProvider = clusterProvider
	daemon.netController.SetClusterProvider(clusterProvider)
	daemon.attachableNetworkLock = locker.New()
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
func NewDaemon(ctx context.Context, config *config.Config, pluginStore *plugin.Store) (daemon *Daemon, err error) {
	// Verify the platform is supported as a daemon
	if !platformSupported {
		return nil, errors.New("the Docker daemon is not supported on this platform")
	}

	registryService, err := registry.NewService(config.ServiceOptions)
	if err != nil {
		return nil, err
	}

	// Ensure that we have a correct root key limit for launching containers.
	if err := modifyRootKeyLimit(); err != nil {
		logrus.Warnf("unable to modify root key limit, number of containers could be limited by this quota: %v", err)
	}

	// Ensure we have compatible and valid configuration options
	if err := verifyDaemonSettings(config); err != nil {
		return nil, err
	}

	// Do we have a disabled network?
	config.DisableBridge = isBridgeNetworkDisabled(config)

	// Setup the resolv.conf
	setupResolvConf(config)

	// Validate platform-specific requirements
	if err := checkSystem(); err != nil {
		return nil, err
	}

	idMapping, err := setupRemappedRoot(config)
	if err != nil {
		return nil, err
	}
	rootIDs := idMapping.RootPair()
	if err := setupDaemonProcess(config); err != nil {
		return nil, err
	}

	// set up the tmpDir to use a canonical path
	tmp, err := prepareTempDir(config.Root)
	if err != nil {
		return nil, fmt.Errorf("Unable to get the TempDir under %s: %s", config.Root, err)
	}
	realTmp, err := fileutils.ReadSymlinkedDirectory(tmp)
	if err != nil {
		return nil, fmt.Errorf("Unable to get the full path to the TempDir (%s): %s", tmp, err)
	}
	if isWindows {
		if _, err := os.Stat(realTmp); err != nil && os.IsNotExist(err) {
			if err := system.MkdirAll(realTmp, 0700); err != nil {
				return nil, fmt.Errorf("Unable to create the TempDir (%s): %s", realTmp, err)
			}
		}
		os.Setenv("TEMP", realTmp)
		os.Setenv("TMP", realTmp)
	} else {
		os.Setenv("TMPDIR", realTmp)
	}

	d := &Daemon{
		configStore: config,
		PluginStore: pluginStore,
		startupDone: make(chan struct{}),
	}

	// Ensure the daemon is properly shutdown if there is a failure during
	// initialization
	defer func() {
		if err != nil {
			if err := d.Shutdown(ctx); err != nil {
				logrus.Error(err)
			}
		}
	}()

	if err := d.setGenericResources(config); err != nil {
		return nil, err
	}
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

	if err := configureMaxThreads(config); err != nil {
		logrus.Warnf("Failed to configure golang's threads limit: %v", err)
	}

	// ensureDefaultAppArmorProfile does nothing if apparmor is disabled
	if err := ensureDefaultAppArmorProfile(); err != nil {
		logrus.Errorf(err.Error())
	}

	daemonRepo := filepath.Join(config.Root, "containers")
	if err := idtools.MkdirAllAndChown(daemonRepo, 0710, idtools.Identity{
		UID: idtools.CurrentIdentity().UID,
		GID: rootIDs.GID,
	}); err != nil {
		return nil, err
	}

	// Create the directory where we'll store the runtime scripts (i.e. in
	// order to support runtimeArgs)
	daemonRuntimes := filepath.Join(config.Root, "runtimes")
	if err := system.MkdirAll(daemonRuntimes, 0700); err != nil {
		return nil, err
	}
	if err := d.loadRuntimes(); err != nil {
		return nil, err
	}

	if isWindows {
		if err := system.MkdirAll(filepath.Join(config.Root, "credentialspecs"), 0); err != nil {
			return nil, err
		}
	}

	d.registryService = registryService
	logger.RegisterPluginGetter(d.PluginStore)

	metricsSockPath, err := d.listenMetricsSock()
	if err != nil {
		return nil, err
	}
	registerMetricsPluginCallback(d.PluginStore, metricsSockPath)

	backoffConfig := backoff.DefaultConfig
	backoffConfig.MaxDelay = 3 * time.Second
	connParams := grpc.ConnectParams{
		Backoff: backoffConfig,
	}
	gopts := []grpc.DialOption{
		// WithBlock makes sure that the following containerd request
		// is reliable.
		//
		// NOTE: In one edge case with high load pressure, kernel kills
		// dockerd, containerd and containerd-shims caused by OOM.
		// When both dockerd and containerd restart, but containerd
		// will take time to recover all the existing containers. Before
		// containerd serving, dockerd will failed with gRPC error.
		// That bad thing is that restore action will still ignore the
		// any non-NotFound errors and returns running state for
		// already stopped container. It is unexpected behavior. And
		// we need to restart dockerd to make sure that anything is OK.
		//
		// It is painful. Add WithBlock can prevent the edge case. And
		// n common case, the containerd will be serving in shortly.
		// It is not harm to add WithBlock for containerd connection.
		grpc.WithBlock(),

		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(connParams),
		grpc.WithContextDialer(dialer.ContextDialer),

		// TODO(stevvooe): We may need to allow configuration of this on the client.
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaults.DefaultMaxRecvMsgSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(defaults.DefaultMaxSendMsgSize)),
	}

	if config.ContainerdAddr != "" {
		d.containerdCli, err = containerd.New(config.ContainerdAddr, containerd.WithDefaultNamespace(config.ContainerdNamespace), containerd.WithDialOpts(gopts), containerd.WithTimeout(60*time.Second))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to dial %q", config.ContainerdAddr)
		}
	}

	createPluginExec := func(m *plugin.Manager) (plugin.Executor, error) {
		var pluginCli *containerd.Client

		if config.ContainerdAddr != "" {
			pluginCli, err = containerd.New(config.ContainerdAddr, containerd.WithDefaultNamespace(config.ContainerdPluginNamespace), containerd.WithDialOpts(gopts), containerd.WithTimeout(60*time.Second))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to dial %q", config.ContainerdAddr)
			}
		}

		var rt types.Runtime
		if runtime.GOOS != "windows" {
			rtPtr, err := d.getRuntime(config.GetDefaultRuntimeName())
			if err != nil {
				return nil, err
			}
			rt = *rtPtr
		}
		return pluginexec.New(ctx, getPluginExecRoot(config.Root), pluginCli, config.ContainerdPluginNamespace, m, rt)
	}

	// Plugin system initialization should happen before restore. Do not change order.
	d.pluginManager, err = plugin.NewManager(plugin.ManagerConfig{
		Root:               filepath.Join(config.Root, "plugins"),
		ExecRoot:           getPluginExecRoot(config.Root),
		Store:              d.PluginStore,
		CreateExecutor:     createPluginExec,
		RegistryService:    registryService,
		LiveRestoreEnabled: config.LiveRestoreEnabled,
		LogPluginEvent:     d.LogPluginEvent, // todo: make private
		AuthzMiddleware:    config.AuthzMiddleware,
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create plugin manager")
	}

	if err := d.setupDefaultLogConfig(); err != nil {
		return nil, err
	}

	d.volumes, err = volumesservice.NewVolumeService(config.Root, d.PluginStore, rootIDs, d)
	if err != nil {
		return nil, err
	}

	// Try to preserve the daemon ID (which is the trust-key's ID) when upgrading
	// an existing installation; this is a "best-effort".
	idPath := filepath.Join(config.Root, "engine-id")
	err = migrateTrustKeyID(config.TrustKeyPath, idPath)
	if err != nil {
		logrus.WithError(err).Warnf("unable to migrate engine ID; a new engine ID will be generated")
	}

	// Check if Devices cgroup is mounted, it is hard requirement for container security,
	// on Linux.
	//
	// Important: we call getSysInfo() directly here, without storing the results,
	// as networking has not yet been set up, so we only have partial system info
	// at this point.
	//
	// TODO(thaJeztah) add a utility to only collect the CgroupDevicesEnabled information
	if runtime.GOOS == "linux" && !userns.RunningInUserNS() && !getSysInfo(d).CgroupDevicesEnabled {
		return nil, errors.New("Devices cgroup isn't mounted")
	}

	d.id, err = loadOrCreateID(idPath)
	if err != nil {
		return nil, err
	}
	d.repository = daemonRepo
	d.containers = container.NewMemoryStore()
	if d.containersReplica, err = container.NewViewDB(); err != nil {
		return nil, err
	}
	d.execCommands = container.NewExecStore()
	d.statsCollector = d.newStatsCollector(1 * time.Second)

	d.EventsService = events.New()
	d.root = config.Root
	d.idMapping = idMapping

	d.linkIndex = newLinkIndex()

	// On Windows we don't support the environment variable, or a user supplied graphdriver
	// Unix platforms however run a single graphdriver for all containers, and it can
	// be set through an environment variable, a daemon start parameter, or chosen through
	// initialization of the layerstore through driver priority order for example.
	driverName := os.Getenv("DOCKER_DRIVER")
	if isWindows {
		driverName = "windowsfilter"
	} else if driverName != "" {
		logrus.Infof("Setting the storage driver from the $DOCKER_DRIVER environment variable (%s)", driverName)
	} else {
		driverName = config.GraphDriver
	}

	if d.UsesSnapshotter() {
		// FIXME(thaJeztah): implement automatic snapshotter-selection similar to graph-driver selection; see https://github.com/moby/moby/issues/44076
		if driverName == "" {
			driverName = containerd.DefaultSnapshotter
		}

		// Configure and validate the kernels security support. Note this is a Linux/FreeBSD
		// operation only, so it is safe to pass *just* the runtime OS graphdriver.
		if err := configureKernelSecuritySupport(config, driverName); err != nil {
			return nil, err
		}
		d.imageService = ctrd.NewService(d.containerdCli, driverName)
	} else {
		layerStore, err := layer.NewStoreFromOptions(layer.StoreOptions{
			Root:                      config.Root,
			MetadataStorePathTemplate: filepath.Join(config.Root, "image", "%s", "layerdb"),
			GraphDriver:               driverName,
			GraphDriverOptions:        config.GraphOptions,
			IDMapping:                 idMapping,
			PluginGetter:              d.PluginStore,
			ExperimentalEnabled:       config.Experimental,
		})
		if err != nil {
			return nil, err
		}

		// Configure and validate the kernels security support. Note this is a Linux/FreeBSD
		// operation only, so it is safe to pass *just* the runtime OS graphdriver.
		if err := configureKernelSecuritySupport(config, layerStore.DriverName()); err != nil {
			return nil, err
		}

		imageRoot := filepath.Join(config.Root, "image", layerStore.DriverName())
		ifs, err := image.NewFSStoreBackend(filepath.Join(imageRoot, "imagedb"))
		if err != nil {
			return nil, err
		}

		// We have a single tag/reference store for the daemon globally. However, it's
		// stored under the graphdriver. On host platforms which only support a single
		// container OS, but multiple selectable graphdrivers, this means depending on which
		// graphdriver is chosen, the global reference store is under there. For
		// platforms which support multiple container operating systems, this is slightly
		// more problematic as where does the global ref store get located? Fortunately,
		// for Windows, which is currently the only daemon supporting multiple container
		// operating systems, the list of graphdrivers available isn't user configurable.
		// For backwards compatibility, we just put it under the windowsfilter
		// directory regardless.
		refStoreLocation := filepath.Join(imageRoot, `repositories.json`)
		rs, err := refstore.NewReferenceStore(refStoreLocation)
		if err != nil {
			return nil, fmt.Errorf("Couldn't create reference store repository: %s", err)
		}
		d.ReferenceStore = rs

		imageStore, err := image.NewImageStore(ifs, layerStore)
		if err != nil {
			return nil, err
		}

		distributionMetadataStore, err := dmetadata.NewFSMetadataStore(filepath.Join(imageRoot, "distribution"))
		if err != nil {
			return nil, err
		}

		imgSvcConfig := images.ImageServiceConfig{
			ContainerStore:            d.containers,
			DistributionMetadataStore: distributionMetadataStore,
			EventsService:             d.EventsService,
			ImageStore:                imageStore,
			LayerStore:                layerStore,
			MaxConcurrentDownloads:    config.MaxConcurrentDownloads,
			MaxConcurrentUploads:      config.MaxConcurrentUploads,
			MaxDownloadAttempts:       config.MaxDownloadAttempts,
			ReferenceStore:            rs,
			RegistryService:           registryService,
			ContentNamespace:          config.ContainerdNamespace,
		}

		// This is a temporary environment variables used in CI to allow pushing
		// manifest v2 schema 1 images to test-registries used for testing *pulling*
		// these images.
		if os.Getenv("DOCKER_ALLOW_SCHEMA1_PUSH_DONOTUSE") != "" {
			imgSvcConfig.TrustKey, err = loadOrCreateTrustKey(config.TrustKeyPath)
			if err != nil {
				return nil, err
			}
			if err = system.MkdirAll(filepath.Join(config.Root, "trust"), 0700); err != nil {
				return nil, err
			}
		}

		// containerd is not currently supported with Windows.
		// So sometimes d.containerdCli will be nil
		// In that case we'll create a local content store... but otherwise we'll use containerd
		if d.containerdCli != nil {
			imgSvcConfig.Leases = d.containerdCli.LeasesService()
			imgSvcConfig.ContentStore = d.containerdCli.ContentStore()
		} else {
			cs, lm, err := d.configureLocalContentStore(config.ContainerdNamespace)
			if err != nil {
				return nil, err
			}
			imgSvcConfig.ContentStore = cs
			imgSvcConfig.Leases = lm
		}

		// TODO: imageStore, distributionMetadataStore, and ReferenceStore are only
		// used above to run migration. They could be initialized in ImageService
		// if migration is called from daemon/images. layerStore might move as well.
		d.imageService = images.NewImageService(imgSvcConfig)

		logrus.Debugf("Max Concurrent Downloads: %d", imgSvcConfig.MaxConcurrentDownloads)
		logrus.Debugf("Max Concurrent Uploads: %d", imgSvcConfig.MaxConcurrentUploads)
		logrus.Debugf("Max Download Attempts: %d", imgSvcConfig.MaxDownloadAttempts)
	}

	go d.execCommandGC()

	if err := d.initLibcontainerd(ctx); err != nil {
		return nil, err
	}

	if err := d.restore(ctx); err != nil {
		return nil, err
	}
	close(d.startupDone)

	info := d.SystemInfo()
	for _, w := range info.Warnings {
		logrus.Warn(w)
	}

	engineInfo.WithValues(
		dockerversion.Version,
		dockerversion.GitCommit,
		info.Architecture,
		info.Driver,
		info.KernelVersion,
		info.OperatingSystem,
		info.OSType,
		info.OSVersion,
		info.ID,
	).Set(1)
	engineCpus.Set(float64(info.NCPU))
	engineMemory.Set(float64(info.MemTotal))

	logrus.WithFields(logrus.Fields{
		"version":     dockerversion.Version,
		"commit":      dockerversion.GitCommit,
		"graphdriver": d.ImageService().StorageDriver(),
	}).Info("Docker daemon")

	return d, nil
}

// DistributionServices returns services controlling daemon storage
func (daemon *Daemon) DistributionServices() images.DistributionServices {
	return daemon.imageService.DistributionServices()
}

func (daemon *Daemon) waitForStartupDone() {
	<-daemon.startupDone
}

func (daemon *Daemon) shutdownContainer(c *container.Container) error {
	// If container failed to exit in stopTimeout seconds of SIGTERM, then using the force
	if err := daemon.containerStop(context.TODO(), c, containertypes.StopOptions{}); err != nil {
		return fmt.Errorf("Failed to stop container %s with error: %v", c.ID, err)
	}

	// Wait without timeout for the container to exit.
	// Ignore the result.
	<-c.Wait(context.Background(), container.WaitConditionNotRunning)
	return nil
}

// ShutdownTimeout returns the timeout (in seconds) before containers are forcibly
// killed during shutdown. The default timeout can be configured both on the daemon
// and per container, and the longest timeout will be used. A grace-period of
// 5 seconds is added to the configured timeout.
//
// A negative (-1) timeout means "indefinitely", which means that containers
// are not forcibly killed, and the daemon shuts down after all containers exit.
func (daemon *Daemon) ShutdownTimeout() int {
	shutdownTimeout := daemon.configStore.ShutdownTimeout
	if shutdownTimeout < 0 {
		return -1
	}
	if daemon.containers == nil {
		return shutdownTimeout
	}

	graceTimeout := 5
	for _, c := range daemon.containers.List() {
		stopTimeout := c.StopTimeout()
		if stopTimeout < 0 {
			return -1
		}
		if stopTimeout+graceTimeout > shutdownTimeout {
			shutdownTimeout = stopTimeout + graceTimeout
		}
	}
	return shutdownTimeout
}

// Shutdown stops the daemon.
func (daemon *Daemon) Shutdown(ctx context.Context) error {
	daemon.shutdown = true
	// Keep mounts and networking running on daemon shutdown if
	// we are to keep containers running and restore them.

	if daemon.configStore.LiveRestoreEnabled && daemon.containers != nil {
		// check if there are any running containers, if none we should do some cleanup
		if ls, err := daemon.Containers(ctx, &types.ContainerListOptions{}); len(ls) != 0 || err != nil {
			// metrics plugins still need some cleanup
			daemon.cleanupMetricsPlugins()
			return nil
		}
	}

	if daemon.containers != nil {
		logrus.Debugf("daemon configured with a %d seconds minimum shutdown timeout", daemon.configStore.ShutdownTimeout)
		logrus.Debugf("start clean shutdown of all containers with a %d seconds timeout...", daemon.ShutdownTimeout())
		daemon.containers.ApplyAll(func(c *container.Container) {
			if !c.IsRunning() {
				return
			}
			log := logrus.WithField("container", c.ID)
			log.Debug("shutting down container")
			if err := daemon.shutdownContainer(c); err != nil {
				log.WithError(err).Error("failed to shut down container")
				return
			}
			if mountid, err := daemon.imageService.GetLayerMountID(c.ID); err == nil {
				daemon.cleanupMountsByID(mountid)
			}
			log.Debugf("shut down container")
		})
	}

	if daemon.volumes != nil {
		if err := daemon.volumes.Shutdown(); err != nil {
			logrus.Errorf("Error shutting down volume store: %v", err)
		}
	}

	if daemon.imageService != nil {
		if err := daemon.imageService.Cleanup(); err != nil {
			logrus.Error(err)
		}
	}

	// If we are part of a cluster, clean up cluster's stuff
	if daemon.clusterProvider != nil {
		logrus.Debugf("start clean shutdown of cluster resources...")
		daemon.DaemonLeavesCluster()
	}

	daemon.cleanupMetricsPlugins()

	// Shutdown plugins after containers and layerstore. Don't change the order.
	daemon.pluginShutdown()

	// trigger libnetwork Stop only if it's initialized
	if daemon.netController != nil {
		daemon.netController.Stop()
	}

	if daemon.containerdCli != nil {
		daemon.containerdCli.Close()
	}

	if daemon.mdDB != nil {
		daemon.mdDB.Close()
	}

	return daemon.cleanupMounts()
}

// Mount sets container.BaseFS
// (is it not set coming in? why is it unset?)
func (daemon *Daemon) Mount(container *container.Container) error {
	if daemon.UsesSnapshotter() {
		return nil
	}
	if container.RWLayer == nil {
		return errors.New("RWLayer of container " + container.ID + " is unexpectedly nil")
	}
	dir, err := container.RWLayer.Mount(container.GetMountLabel())
	if err != nil {
		return err
	}
	logrus.WithField("container", container.ID).Debugf("container mounted via layerStore: %v", dir)

	if container.BaseFS != "" && container.BaseFS != dir {
		// The mount path reported by the graph driver should always be trusted on Windows, since the
		// volume path for a given mounted layer may change over time.  This should only be an error
		// on non-Windows operating systems.
		if runtime.GOOS != "windows" {
			daemon.Unmount(container)
			return fmt.Errorf("driver %s is returning inconsistent paths for container %s ('%s' then '%s')",
				container.Driver, container.ID, container.BaseFS, dir)
		}
	}
	container.BaseFS = dir // TODO: combine these fields
	return nil
}

// Unmount unsets the container base filesystem
func (daemon *Daemon) Unmount(container *container.Container) error {
	if daemon.UsesSnapshotter() {
		return nil
	}
	if container.RWLayer == nil {
		return errors.New("RWLayer of container " + container.ID + " is unexpectedly nil")
	}
	if err := container.RWLayer.Unmount(); err != nil {
		logrus.WithField("container", container.ID).WithError(err).Error("error unmounting container")
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

// prepareTempDir prepares and returns the default directory to use
// for temporary files.
// If it doesn't exist, it is created. If it exists, its content is removed.
func prepareTempDir(rootDir string) (string, error) {
	var tmpDir string
	if tmpDir = os.Getenv("DOCKER_TMPDIR"); tmpDir == "" {
		tmpDir = filepath.Join(rootDir, "tmp")
		newName := tmpDir + "-old"
		if err := os.Rename(tmpDir, newName); err == nil {
			go func() {
				if err := os.RemoveAll(newName); err != nil {
					logrus.Warnf("failed to delete old tmp directory: %s", newName)
				}
			}()
		} else if !os.IsNotExist(err) {
			logrus.Warnf("failed to rename %s for background deletion: %s. Deleting synchronously", tmpDir, err)
			if err := os.RemoveAll(tmpDir); err != nil {
				logrus.Warnf("failed to delete old tmp directory: %s", tmpDir)
			}
		}
	}
	return tmpDir, idtools.MkdirAllAndChown(tmpDir, 0700, idtools.CurrentIdentity())
}

func (daemon *Daemon) setGenericResources(conf *config.Config) error {
	genericResources, err := config.ParseGenericResources(conf.NodeGenericResources)
	if err != nil {
		return err
	}

	daemon.genericResources = genericResources

	return nil
}

// IsShuttingDown tells whether the daemon is shutting down or not
func (daemon *Daemon) IsShuttingDown() bool {
	return daemon.shutdown
}

func isBridgeNetworkDisabled(conf *config.Config) bool {
	return conf.BridgeConfig.Iface == config.DisableNetworkBridge
}

func (daemon *Daemon) networkOptions(pg plugingetter.PluginGetter, activeSandboxes map[string]interface{}) ([]nwconfig.Option, error) {
	options := []nwconfig.Option{}
	if daemon.configStore == nil {
		return options, nil
	}
	conf := daemon.configStore
	dd := runconfig.DefaultDaemonNetworkMode()

	options = []nwconfig.Option{
		nwconfig.OptionDataDir(conf.Root),
		nwconfig.OptionExecRoot(conf.GetExecRoot()),
		nwconfig.OptionDefaultDriver(string(dd)),
		nwconfig.OptionDefaultNetwork(dd.NetworkName()),
		nwconfig.OptionLabels(conf.Labels),
		nwconfig.OptionNetworkControlPlaneMTU(conf.NetworkControlPlaneMTU),
		driverOptions(conf),
	}

	if len(conf.NetworkConfig.DefaultAddressPools.Value()) > 0 {
		options = append(options, nwconfig.OptionDefaultAddressPoolConfig(conf.NetworkConfig.DefaultAddressPools.Value()))
	}
	if conf.LiveRestoreEnabled && len(activeSandboxes) != 0 {
		options = append(options, nwconfig.OptionActiveSandboxes(activeSandboxes))
	}
	if pg != nil {
		options = append(options, nwconfig.OptionPluginGetter(pg))
	}

	return options, nil
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

	idMapping, err := setupRemappedRoot(config)
	if err != nil {
		return err
	}
	return setupDaemonRoot(config, realRoot, idMapping.RootPair())
}

// checkpointAndSave grabs a container lock to safely call container.CheckpointTo
func (daemon *Daemon) checkpointAndSave(container *container.Container) error {
	container.Lock()
	defer container.Unlock()
	if err := container.CheckpointTo(daemon.containersReplica); err != nil {
		return fmt.Errorf("Error saving container state: %v", err)
	}
	return nil
}

// because the CLI sends a -1 when it wants to unset the swappiness value
// we need to clear it on the server side
func fixMemorySwappiness(resources *containertypes.Resources) {
	if resources.MemorySwappiness != nil && *resources.MemorySwappiness == -1 {
		resources.MemorySwappiness = nil
	}
}

// GetAttachmentStore returns current attachment store associated with the daemon
func (daemon *Daemon) GetAttachmentStore() *network.AttachmentStore {
	return &daemon.attachmentStore
}

// IdentityMapping returns uid/gid mapping or a SID (in the case of Windows) for the builder
func (daemon *Daemon) IdentityMapping() idtools.IdentityMapping {
	return daemon.idMapping
}

// ImageService returns the Daemon's ImageService
func (daemon *Daemon) ImageService() ImageService {
	return daemon.imageService
}

// BuilderBackend returns the backend used by builder
func (daemon *Daemon) BuilderBackend() builder.Backend {
	return struct {
		*Daemon
		ImageService
	}{daemon, daemon.imageService}
}

// RawSysInfo returns *sysinfo.SysInfo .
func (daemon *Daemon) RawSysInfo() *sysinfo.SysInfo {
	daemon.sysInfoOnce.Do(func() {
		// We check if sysInfo is not set here, to allow some test to
		// override the actual sysInfo.
		if daemon.sysInfo == nil {
			daemon.sysInfo = getSysInfo(daemon)
		}
	})

	return daemon.sysInfo
}
