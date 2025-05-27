// TODO(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/internal/metrics"
	"github.com/docker/docker/internal/multierror"
	"github.com/docker/docker/internal/otelutil"
	"github.com/docker/docker/runconfig"
	"github.com/moby/sys/user"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/tonistiigi/go-archvariant"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type createOpts struct {
	params                  backend.ContainerCreateConfig
	managed                 bool
	ignoreImagesArgsEscaped bool
}

// CreateManagedContainer creates a container that is managed by a Service
func (daemon *Daemon) CreateManagedContainer(ctx context.Context, params backend.ContainerCreateConfig) (containertypes.CreateResponse, error) {
	return daemon.containerCreate(ctx, daemon.config(), createOpts{
		params:  params,
		managed: true,
	})
}

// ContainerCreate creates a regular container
func (daemon *Daemon) ContainerCreate(ctx context.Context, params backend.ContainerCreateConfig) (containertypes.CreateResponse, error) {
	return daemon.containerCreate(ctx, daemon.config(), createOpts{
		params: params,
	})
}

// ContainerCreateIgnoreImagesArgsEscaped creates a regular container. This is called from the builder RUN case
// and ensures that we do not take the images ArgsEscaped
func (daemon *Daemon) ContainerCreateIgnoreImagesArgsEscaped(ctx context.Context, params backend.ContainerCreateConfig) (containertypes.CreateResponse, error) {
	return daemon.containerCreate(ctx, daemon.config(), createOpts{
		params:                  params,
		ignoreImagesArgsEscaped: true,
	})
}

func (daemon *Daemon) containerCreate(ctx context.Context, daemonCfg *configStore, opts createOpts) (_ containertypes.CreateResponse, retErr error) {
	ctx, span := otel.Tracer("").Start(ctx, "daemon.containerCreate", trace.WithAttributes(
		labelsAsOTelAttributes(opts.params.Config.Labels)...,
	))
	defer func() {
		otelutil.RecordStatus(span, retErr)
		span.End()
	}()

	start := time.Now()
	if opts.params.Config == nil {
		return containertypes.CreateResponse{}, errdefs.InvalidParameter(runconfig.ErrEmptyConfig)
	}

	// Normalize some defaults. Doing this "ad-hoc" here for now, as there's
	// only one field to migrate, but we should consider having a better
	// location for this (and decide where in the flow would be most appropriate).
	//
	// TODO(thaJeztah): we should have a more visible, more canonical location for this.
	if opts.params.HostConfig != nil && opts.params.HostConfig.RestartPolicy.Name == "" {
		// Set the default restart-policy ("none") if no restart-policy was set.
		opts.params.HostConfig.RestartPolicy.Name = containertypes.RestartPolicyDisabled
	}

	warnings, err := daemon.verifyContainerSettings(daemonCfg, opts.params.HostConfig, opts.params.Config, false)
	if err != nil {
		return containertypes.CreateResponse{Warnings: warnings}, errdefs.InvalidParameter(err)
	}

	if opts.params.Platform == nil && opts.params.Config.Image != "" {
		img, err := daemon.imageService.GetImage(ctx, opts.params.Config.Image, backend.GetImageOpts{})
		if err != nil {
			return containertypes.CreateResponse{}, err
		}
		if img != nil {
			p := maximumSpec()
			imgPlat := ocispec.Platform{
				OS:           img.OS,
				Architecture: img.Architecture,
				Variant:      img.Variant,
			}

			if !images.OnlyPlatformWithFallback(p).Match(imgPlat) {
				warnings = append(warnings, fmt.Sprintf("The requested image's platform (%s) does not match the detected host platform (%s) and no specific platform was requested", platforms.FormatAll(imgPlat), platforms.FormatAll(p)))
			}
		}
	}

	err = daemon.validateNetworkingConfig(opts.params.NetworkingConfig)
	if err != nil {
		return containertypes.CreateResponse{Warnings: warnings}, errdefs.InvalidParameter(err)
	}

	if opts.params.HostConfig == nil {
		opts.params.HostConfig = &containertypes.HostConfig{}
	}
	err = daemon.adaptContainerSettings(&daemonCfg.Config, opts.params.HostConfig)
	if err != nil {
		return containertypes.CreateResponse{Warnings: warnings}, errdefs.InvalidParameter(err)
	}

	ctr, err := daemon.create(ctx, &daemonCfg.Config, opts)
	if err != nil {
		return containertypes.CreateResponse{Warnings: warnings}, err
	}
	metrics.ContainerActions.WithValues("create").UpdateSince(start)

	if warnings == nil {
		warnings = make([]string, 0) // Create an empty slice to avoid https://github.com/moby/moby/issues/38222
	}

	return containertypes.CreateResponse{ID: ctr.ID, Warnings: warnings}, nil
}

var (
	containerLabelsFilter     []string
	containerLabelsFilterOnce sync.Once
)

func labelsAsOTelAttributes(labels map[string]string) []attribute.KeyValue {
	containerLabelsFilterOnce.Do(func() {
		containerLabelsFilter = strings.Split(os.Getenv("DOCKER_CONTAINER_LABELS_FILTER"), ",")
	})

	// This env var is a comma-separated list of labels to be included in the
	// OTel span attributes. The labels are prefixed with "label." to avoid
	// collision with other attributes.
	//
	// Note that, this is an experimental env var that might be removed
	// unceremoniously at any point in time.
	attrs := make([]attribute.KeyValue, 0, len(containerLabelsFilter))
	for _, k := range containerLabelsFilter {
		if v, ok := labels[k]; ok {
			attrs = append(attrs, attribute.String("label."+k, v))
		}
	}
	return attrs
}

// Create creates a new container from the given configuration with a given name.
func (daemon *Daemon) create(ctx context.Context, daemonCfg *config.Config, opts createOpts) (retC *container.Container, retErr error) {
	var (
		ctr         *container.Container
		img         *image.Image
		imgManifest *ocispec.Descriptor
		imgID       image.ID
		err         error
		platform    = platforms.DefaultSpec()
	)

	if opts.params.Config.Image != "" {
		img, err = daemon.imageService.GetImage(ctx, opts.params.Config.Image, backend.GetImageOpts{Platform: opts.params.Platform})
		if err != nil {
			return nil, err
		}
		if img.Details != nil {
			imgManifest = img.Details.ManifestDescriptor
		}
		platform = img.Platform()
		imgID = img.ID()
	} else if isWindows {
		platform.OS = "linux" // 'scratch' case.
	}

	// On WCOW, if are not being invoked by the builder to create this container (where
	// ignoreImagesArgEscaped will be true) - if the image already has its arguments escaped,
	// ensure that this is replicated across to the created container to avoid double-escaping
	// of the arguments/command line when the runtime attempts to run the container.
	if platform.OS == "windows" && !opts.ignoreImagesArgsEscaped && img != nil && img.RunConfig().ArgsEscaped {
		opts.params.Config.ArgsEscaped = true
	}

	if err := daemon.mergeAndVerifyConfig(opts.params.Config, img); err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	if err := daemon.mergeAndVerifyLogConfig(&opts.params.HostConfig.LogConfig); err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	if ctr, err = daemon.newContainer(opts.params.Name, platform, opts.params.Config, opts.params.HostConfig, imgID, opts.managed); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			err = daemon.cleanupContainer(ctr, backend.ContainerRmConfig{
				ForceRemove:  true,
				RemoveVolume: true,
			})
			if err != nil {
				log.G(ctx).WithFields(log.Fields{
					"error":     err,
					"container": ctr.ID,
				}).Errorf("failed to cleanup container on create error")
			}
		}
	}()

	if err := daemon.setSecurityOptions(daemonCfg, ctr, opts.params.HostConfig); err != nil {
		return nil, err
	}

	ctr.HostConfig.StorageOpt = opts.params.HostConfig.StorageOpt
	ctr.ImageManifest = imgManifest

	// Set RWLayer for container after mount labels have been set
	rwLayer, err := daemon.imageService.CreateLayer(ctr, setupInitLayer(daemon.idMapping.RootPair()))
	if err != nil {
		return nil, errdefs.System(err)
	}
	ctr.RWLayer = rwLayer

	cuid := os.Getuid()
	_, gid := daemon.IdentityMapping().RootPair()
	if err := user.MkdirAndChown(ctr.Root, 0o710, cuid, gid); err != nil {
		return nil, err
	}
	if err := user.MkdirAndChown(ctr.CheckpointDir(), 0o700, cuid, os.Getegid()); err != nil {
		return nil, err
	}

	if err := daemon.setHostConfig(ctr, opts.params.HostConfig, opts.params.DefaultReadOnlyNonRecursive); err != nil {
		return nil, err
	}

	if err := daemon.createContainerOSSpecificSettings(ctx, ctr, opts.params.Config, opts.params.HostConfig); err != nil {
		return nil, err
	}

	var endpointsConfigs map[string]*networktypes.EndpointSettings
	if opts.params.NetworkingConfig != nil {
		endpointsConfigs = opts.params.NetworkingConfig.EndpointsConfig
	}
	// Make sure NetworkMode has an acceptable value. We do this to ensure
	// backwards API compatibility.
	if ctr.HostConfig != nil && ctr.HostConfig.NetworkMode == "" {
		ctr.HostConfig.NetworkMode = networktypes.NetworkDefault
	}

	daemon.updateContainerNetworkSettings(ctr, endpointsConfigs)
	if err := daemon.register(ctx, ctr); err != nil {
		return nil, err
	}
	metrics.StateCtr.Set(ctr.ID, "stopped")
	daemon.LogContainerEvent(ctr, events.ActionCreate)
	return ctr, nil
}

func toHostConfigSelinuxLabels(labels []string) []string {
	for i, l := range labels {
		labels[i] = "label=" + l
	}
	return labels
}

func (daemon *Daemon) generateSecurityOpt(hostConfig *containertypes.HostConfig) ([]string, error) {
	for _, opt := range hostConfig.SecurityOpt {
		con := strings.Split(opt, "=")
		if con[0] == "label" {
			// Caller overrode SecurityOpts
			return nil, nil
		}
	}
	ipcMode := hostConfig.IpcMode
	pidMode := hostConfig.PidMode
	privileged := hostConfig.Privileged
	if ipcMode.IsHost() || pidMode.IsHost() || privileged {
		return toHostConfigSelinuxLabels(selinux.DisableSecOpt()), nil
	}

	var ipcLabel []string
	var pidLabel []string
	ipcContainer := ipcMode.Container()
	pidContainer := pidMode.Container()
	if ipcContainer != "" {
		c, err := daemon.GetContainer(ipcContainer)
		if err != nil {
			return nil, err
		}
		ipcLabel, err = selinux.DupSecOpt(c.ProcessLabel)
		if err != nil {
			return nil, err
		}
		if pidContainer == "" {
			return toHostConfigSelinuxLabels(ipcLabel), err
		}
	}
	if pidContainer != "" {
		c, err := daemon.GetContainer(pidContainer)
		if err != nil {
			return nil, err
		}

		pidLabel, err = selinux.DupSecOpt(c.ProcessLabel)
		if err != nil {
			return nil, err
		}
		if ipcContainer == "" {
			return toHostConfigSelinuxLabels(pidLabel), err
		}
	}

	if pidLabel != nil && ipcLabel != nil {
		for i := 0; i < len(pidLabel); i++ {
			if pidLabel[i] != ipcLabel[i] {
				return nil, fmt.Errorf("--ipc and --pid containers SELinux labels aren't the same")
			}
		}
		return toHostConfigSelinuxLabels(pidLabel), nil
	}
	return nil, nil
}

func (daemon *Daemon) mergeAndVerifyConfig(config *containertypes.Config, img *image.Image) error {
	if img != nil && img.Config != nil {
		if err := merge(config, img.Config); err != nil {
			return err
		}
	}
	// Reset the Entrypoint if it is [""]
	if len(config.Entrypoint) == 1 && config.Entrypoint[0] == "" {
		config.Entrypoint = nil
	}
	if len(config.Entrypoint) == 0 && len(config.Cmd) == 0 {
		return fmt.Errorf("no command specified")
	}
	return nil
}

// validateNetworkingConfig checks whether a container's NetworkingConfig is valid.
func (daemon *Daemon) validateNetworkingConfig(nwConfig *networktypes.NetworkingConfig) error {
	if nwConfig == nil {
		return nil
	}

	var errs []error
	for k, v := range nwConfig.EndpointsConfig {
		if v == nil {
			errs = append(errs, fmt.Errorf("invalid config for network %s: EndpointsConfig is nil", k))
			continue
		}

		// The referenced network k might not exist when the container is created, so just ignore the error in that case.
		nw, _ := daemon.FindNetwork(k)
		if err := validateEndpointSettings(nw, k, v); err != nil {
			errs = append(errs, fmt.Errorf("invalid config for network %s: %w", k, err))
		}
	}

	if len(errs) > 0 {
		return errdefs.InvalidParameter(multierror.Join(errs...))
	}

	return nil
}

// maximumSpec returns the distribution platform with maximum compatibility for the current node.
func maximumSpec() ocispec.Platform {
	p := platforms.DefaultSpec()
	if p.Architecture == "amd64" {
		p.Variant = archvariant.AMD64Variant()
	}
	return p
}
