package daemon

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/log"
	"github.com/containerd/typeurl/v2"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	networktypes "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/network"
	"github.com/moby/moby/v2/daemon/pkg/oci/caps"
	"github.com/moby/moby/v2/daemon/pkg/opts"
	volumemounts "github.com/moby/moby/v2/daemon/volume/mounts"
	"github.com/moby/moby/v2/errdefs"
	"github.com/moby/sys/signal"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
)

// GetContainer looks for a container using the provided information, which could be
// one of the following inputs from the caller:
//   - A full container ID, which will exact match a container in daemon's list
//   - A container name, which will only exact match via the GetByName() function
//   - A partial container ID prefix (e.g. short ID) of any length that is
//     unique enough to only return a single container object
//     If none of these searches succeed, an error is returned
func (daemon *Daemon) GetContainer(prefixOrName string) (*container.Container, error) {
	if prefixOrName == "" {
		return nil, errors.WithStack(invalidIdentifier(prefixOrName))
	}

	if containerByID := daemon.containers.Get(prefixOrName); containerByID != nil {
		// prefix is an exact match to a full container ID
		return containerByID, nil
	}

	// GetByName will match only an exact name provided; we ignore errors
	if containerByName, _ := daemon.GetByName(prefixOrName); containerByName != nil {
		// prefix is an exact match to a full container Name
		return containerByName, nil
	}

	containerID, err := daemon.containersReplica.GetByPrefix(prefixOrName)
	if err != nil {
		return nil, err
	}
	ctr := daemon.containers.Get(containerID)
	if ctr == nil {
		// Updates to the daemon.containersReplica ViewDB are not atomic
		// or consistent w.r.t. the live daemon.containers Store so
		// while reaching this code path may be indicative of a bug,
		// it is not _necessarily_ the case.
		log.G(context.TODO()).WithField("prefixOrName", prefixOrName).
			WithField("id", containerID).
			Debugf("daemon.GetContainer: container is known to daemon.containersReplica but not daemon.containers")
		return nil, containerNotFound(prefixOrName)
	}
	return ctr, nil
}

func (daemon *Daemon) GetOCISpec(ctx context.Context, id string) (*specs.Spec, error) {
	ctr, err := daemon.containerdClient.LoadContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	info, err := ctr.Info(ctx, containerd.WithoutRefreshedMetadata)
	if err != nil {
		return nil, err
	}
	var ociSpec specs.Spec
	err = typeurl.UnmarshalTo(info.Spec, &ociSpec)
	if err != nil {
		return nil, err
	}
	return &ociSpec, nil
}

// Load reads the contents of a container from disk
// This is typically done at startup.
func (daemon *Daemon) load(id string) (*container.Container, error) {
	ctr := container.NewBaseContainer(id, filepath.Join(daemon.repository, id))
	if err := ctr.FromDisk(); err != nil {
		return nil, err
	}
	selinux.ReserveLabel(ctr.ProcessLabel)

	if ctr.ID != id {
		return ctr, fmt.Errorf("Container %s is stored at %s", ctr.ID, id)
	}

	return ctr, nil
}

// register makes a container object usable by the daemon as [container.Container.ID].
func (daemon *Daemon) register(ctx context.Context, c *container.Container) error {
	// Attach to stdout and stderr
	if c.Config.OpenStdin {
		c.StreamConfig.NewInputPipes()
	} else {
		c.StreamConfig.NewNopInputPipe()
	}

	// once in the memory store it is visible to other goroutines
	// grab a Lock until it has been checkpointed to avoid races
	c.Lock()
	defer c.Unlock()

	// FIXME(thaJeztah): this logic may not be atomic:
	//
	// - daemon.containers.Add does not promise to "add", allows overwriting a container with the given ID.
	// - c.CheckpointTo may fail, in which case we registered a container, but failed to write to disk
	//
	// We should consider:
	//
	// - changing the signature of containers.Add to return an error if the
	//   given ID exists (potentially adding an alternative to "set" / "update")
	// - adding a defer to rollback the "Add" when failing to CheckPoint.
	daemon.containers.Add(c.ID, c)
	return c.CheckpointTo(ctx, daemon.containersReplica)
}

func (daemon *Daemon) newContainer(name string, platform ocispec.Platform, config *containertypes.Config, hostConfig *containertypes.HostConfig, imgID image.ID, managed bool) (*container.Container, error) {
	var (
		id  string
		err error
	)
	id, name, err = daemon.generateIDAndName(name)
	if err != nil {
		return nil, err
	}

	if config.Hostname == "" {
		if hostConfig.NetworkMode.IsHost() {
			config.Hostname, err = os.Hostname()
			if err != nil {
				return nil, errdefs.System(err)
			}
		} else {
			// default hostname is the container's short-ID
			config.Hostname = id[:12]
		}
	}
	entrypoint, args := getEntrypointAndArgs(config.Entrypoint, config.Cmd)

	base := container.NewBaseContainer(id, filepath.Join(daemon.repository, id))
	base.Created = time.Now().UTC()
	base.Managed = managed
	base.Path = entrypoint
	base.Args = args // FIXME: de-duplicate from config
	base.Config = config
	base.HostConfig = hostConfig
	base.ImageID = imgID
	base.NetworkSettings = &network.Settings{}
	base.Name = name
	base.Driver = daemon.imageService.StorageDriver()
	base.ImagePlatform = platform
	base.OS = platform.OS //nolint:staticcheck // ignore SA1019: field is deprecated, but still set for compatibility
	return base, err
}

func getEntrypointAndArgs(configEntrypoint, configCmd []string) (string, []string) {
	if len(configEntrypoint) == 0 {
		return configCmd[0], configCmd[1:]
	}
	return configEntrypoint[0], append(configEntrypoint[1:], configCmd...)
}

// GetByName returns a container given a name.
func (daemon *Daemon) GetByName(name string) (*container.Container, error) {
	if name == "" {
		return nil, errors.New("No container name supplied")
	}
	fullName := name
	if name[0] != '/' {
		fullName = "/" + name
	}
	id, err := daemon.containersReplica.Snapshot().GetID(fullName)
	if err != nil {
		return nil, fmt.Errorf("Could not find entity for %s", name)
	}
	e := daemon.containers.Get(id)
	if e == nil {
		return nil, fmt.Errorf("Could not find container for entity id %s", id)
	}
	return e, nil
}

// GetDependentContainers returns a list of containers that depend on the given container.
// Dependencies are determined by:
//   - Network mode dependencies (--network=container:xxx)
//   - Legacy container links (--link)
//
// This is primarily used during daemon startup to determine container startup order,
// ensuring that dependent containers are started after their dependencies are running.
// Upon error, it returns the last known dependent containers, which may be empty.
func (daemon *Daemon) GetDependentContainers(c *container.Container) []*container.Container {
	var dependentContainers []*container.Container

	if c.HostConfig.NetworkMode.IsContainer() {
		// If the container is using a network mode that depends on another container,
		// we need to find that container and add it to the dependency map.
		dependencyContainer, err := daemon.GetContainer(c.HostConfig.NetworkMode.ConnectedContainer())
		if err != nil {
			log.G(context.TODO()).WithError(err).Errorf("Could not find dependent container for %s", c.ID)
			return dependentContainers
		}
		dependentContainers = append(dependentContainers, dependencyContainer)
	}

	return append(dependentContainers, slices.Collect(maps.Values(daemon.linkIndex.children(c)))...)
}

func (daemon *Daemon) setSecurityOptions(cfg *config.Config, container *container.Container) error {
	container.Lock()
	defer container.Unlock()
	return daemon.parseSecurityOpt(cfg, &container.SecurityOptions, container.HostConfig)
}

// verifyContainerSettings performs validation of the hostconfig and config
// structures.
func (daemon *Daemon) verifyContainerSettings(daemonCfg *configStore, hostConfig *containertypes.HostConfig, config *containertypes.Config, update bool) (warnings []string, _ error) {
	// First perform verification of settings common across all platforms.
	if err := validateContainerConfig(config); err != nil {
		return nil, err
	}

	warns, err := validateHostConfig(hostConfig)
	warnings = append(warnings, warns...)
	if err != nil {
		return warnings, err
	}

	// Now do platform-specific verification
	warns, err = verifyPlatformContainerSettings(daemon, daemonCfg, hostConfig, update)
	warnings = append(warnings, warns...)

	return warnings, err
}

func validateContainerConfig(config *containertypes.Config) error {
	if config == nil {
		return nil
	}
	if err := translateWorkingDir(config); err != nil {
		return err
	}
	if config.StopSignal != "" {
		if _, err := signal.ParseSignal(config.StopSignal); err != nil {
			return err
		}
	}
	// Validate if Env contains empty variable or not (e.g., ``, `=foo`)
	for _, env := range config.Env {
		if _, err := opts.ValidateEnv(env); err != nil {
			return err
		}
	}
	return validateHealthCheck(config.Healthcheck)
}

func validateHostConfig(hostConfig *containertypes.HostConfig) (warnings []string, _ error) {
	if hostConfig == nil {
		return nil, nil
	}

	if hostConfig.AutoRemove && !hostConfig.RestartPolicy.IsNone() {
		return warnings, errors.Errorf("can't create 'AutoRemove' container with restart policy")
	}
	// Validate mounts; check if host directories still exist
	parser := volumemounts.NewParser()
	for _, c := range hostConfig.Mounts {
		cfg := c

		if cfg.Type == mount.TypeImage {
			warnings = append(warnings, "Image mount is an experimental feature")
		}

		if err := parser.ValidateMountConfig(&cfg); err != nil {
			return warnings, err
		}
	}
	for _, extraHost := range hostConfig.ExtraHosts {
		if _, err := opts.ValidateExtraHost(extraHost); err != nil {
			return warnings, err
		}
	}
	if err := validatePortBindings(hostConfig.PortBindings); err != nil {
		return warnings, err
	}
	if err := containertypes.ValidateRestartPolicy(hostConfig.RestartPolicy); err != nil {
		return warnings, err
	}
	if err := validateCapabilities(hostConfig); err != nil {
		return warnings, err
	}
	if !hostConfig.Isolation.IsValid() {
		return warnings, errors.Errorf("invalid isolation '%s' on %s", hostConfig.Isolation, runtime.GOOS)
	}
	for k := range hostConfig.Annotations {
		if k == "" {
			return warnings, errors.Errorf("invalid Annotations: the empty string is not permitted as an annotation key")
		}
	}
	return warnings, nil
}

func validateCapabilities(hostConfig *containertypes.HostConfig) error {
	if _, err := caps.NormalizeLegacyCapabilities(hostConfig.CapAdd); err != nil {
		return errors.Wrap(err, "invalid CapAdd")
	}
	if _, err := caps.NormalizeLegacyCapabilities(hostConfig.CapDrop); err != nil {
		return errors.Wrap(err, "invalid CapDrop")
	}
	// TODO consider returning warnings if "Privileged" is combined with Capabilities, CapAdd and/or CapDrop
	return nil
}

// validateHealthCheck validates the healthcheck params of Config
func validateHealthCheck(healthConfig *containertypes.HealthConfig) error {
	if healthConfig == nil {
		return nil
	}
	if healthConfig.Interval != 0 && healthConfig.Interval < containertypes.MinimumDuration {
		return errors.Errorf("Interval in Healthcheck cannot be less than %s", containertypes.MinimumDuration)
	}
	if healthConfig.Timeout != 0 && healthConfig.Timeout < containertypes.MinimumDuration {
		return errors.Errorf("Timeout in Healthcheck cannot be less than %s", containertypes.MinimumDuration)
	}
	if healthConfig.Retries < 0 {
		return errors.Errorf("Retries in Healthcheck cannot be negative")
	}
	if healthConfig.StartPeriod != 0 && healthConfig.StartPeriod < containertypes.MinimumDuration {
		return errors.Errorf("StartPeriod in Healthcheck cannot be less than %s", containertypes.MinimumDuration)
	}
	if healthConfig.StartInterval != 0 && healthConfig.StartInterval < containertypes.MinimumDuration {
		return errors.Errorf("StartInterval in Healthcheck cannot be less than %s", containertypes.MinimumDuration)
	}
	return nil
}

func validatePortBindings(ports networktypes.PortMap) error {
	for port := range ports {
		if !port.IsValid() || port.Num() == 0 {
			return errors.Errorf("invalid port specification: %q", port.String())
		}

		for _, pb := range ports[port] {
			if pb.HostPort == "" {
				// Empty HostPort means to map to an ephemeral port.
				continue
			}

			if _, err := networktypes.ParsePortRange(pb.HostPort); err != nil {
				return errors.Errorf("invalid port specification: %q", pb.HostPort)
			}
		}
	}
	return nil
}

// translateWorkingDir translates the working-dir for the target platform,
// and returns an error if the given path is not an absolute path.
func translateWorkingDir(config *containertypes.Config) error {
	if config.WorkingDir == "" {
		return nil
	}
	wd := filepath.FromSlash(config.WorkingDir) // Ensure in platform semantics
	if !filepath.IsAbs(wd) && !strings.HasPrefix(wd, string(os.PathSeparator)) {
		return fmt.Errorf("the working directory '%s' is invalid, it needs to be an absolute path", config.WorkingDir)
	}
	config.WorkingDir = filepath.Clean(wd)
	return nil
}
