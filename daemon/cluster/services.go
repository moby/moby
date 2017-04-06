package cluster

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	apierrors "github.com/docker/docker/api/errors"
	apitypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	types "github.com/docker/docker/api/types/swarm"
	timetypes "github.com/docker/docker/api/types/time"
	"github.com/docker/docker/daemon/cluster/convert"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/stdcopy"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	swarmapi "github.com/docker/swarmkit/api"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// GetServices returns all services of a managed swarm cluster.
func (c *Cluster) GetServices(options apitypes.ServiceListOptions) ([]types.Service, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return nil, c.errNoManager(state)
	}

	// We move the accepted filter check here as "mode" filter
	// is processed in the daemon, not in SwarmKit. So it might
	// be good to have accepted file check in the same file as
	// the filter processing (in the for loop below).
	accepted := map[string]bool{
		"name":  true,
		"id":    true,
		"label": true,
		"mode":  true,
	}
	if err := options.Filters.Validate(accepted); err != nil {
		return nil, err
	}
	filters := &swarmapi.ListServicesRequest_Filters{
		NamePrefixes: options.Filters.Get("name"),
		IDPrefixes:   options.Filters.Get("id"),
		Labels:       runconfigopts.ConvertKVStringsToMap(options.Filters.Get("label")),
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := state.controlClient.ListServices(
		ctx,
		&swarmapi.ListServicesRequest{Filters: filters})
	if err != nil {
		return nil, err
	}

	services := []types.Service{}

	for _, service := range r.Services {
		if options.Filters.Include("mode") {
			var mode string
			switch service.Spec.GetMode().(type) {
			case *swarmapi.ServiceSpec_Global:
				mode = "global"
			case *swarmapi.ServiceSpec_Replicated:
				mode = "replicated"
			}

			if !options.Filters.ExactMatch("mode", mode) {
				continue
			}
		}
		services = append(services, convert.ServiceFromGRPC(*service))
	}

	return services, nil
}

// GetService returns a service based on an ID or name.
func (c *Cluster) GetService(input string) (types.Service, error) {
	var service *swarmapi.Service
	if err := c.lockedManagerAction(func(ctx context.Context, state nodeState) error {
		s, err := getService(ctx, state.controlClient, input)
		if err != nil {
			return err
		}
		service = s
		return nil
	}); err != nil {
		return types.Service{}, err
	}
	return convert.ServiceFromGRPC(*service), nil
}

// CreateService creates a new service in a managed swarm cluster.
func (c *Cluster) CreateService(s types.ServiceSpec, encodedAuth string) (*apitypes.ServiceCreateResponse, error) {
	var resp *apitypes.ServiceCreateResponse
	err := c.lockedManagerAction(func(ctx context.Context, state nodeState) error {
		err := c.populateNetworkID(ctx, state.controlClient, &s)
		if err != nil {
			return err
		}

		serviceSpec, err := convert.ServiceSpecToGRPC(s)
		if err != nil {
			return apierrors.NewBadRequestError(err)
		}

		ctnr := serviceSpec.Task.GetContainer()
		if ctnr == nil {
			return errors.New("service does not use container tasks")
		}

		if encodedAuth != "" {
			ctnr.PullOptions = &swarmapi.ContainerSpec_PullOptions{RegistryAuth: encodedAuth}
		}

		// retrieve auth config from encoded auth
		authConfig := &apitypes.AuthConfig{}
		if encodedAuth != "" {
			if err := json.NewDecoder(base64.NewDecoder(base64.URLEncoding, strings.NewReader(encodedAuth))).Decode(authConfig); err != nil {
				logrus.Warnf("invalid authconfig: %v", err)
			}
		}

		resp = &apitypes.ServiceCreateResponse{}

		// pin image by digest
		if os.Getenv("DOCKER_SERVICE_PREFER_OFFLINE_IMAGE") != "1" {
			digestImage, err := c.imageWithDigestString(ctx, ctnr.Image, authConfig)
			if err != nil {
				logrus.Warnf("unable to pin image %s to digest: %s", ctnr.Image, err.Error())
				// warning in the client response should be concise
				resp.Warnings = append(resp.Warnings, digestWarning(ctnr.Image))
			} else if ctnr.Image != digestImage {
				logrus.Debugf("pinning image %s by digest: %s", ctnr.Image, digestImage)
				ctnr.Image = digestImage
			} else {
				logrus.Debugf("creating service using supplied digest reference %s", ctnr.Image)
			}

			// Replace the context with a fresh one.
			// If we timed out while communicating with the
			// registry, then "ctx" will already be expired, which
			// would cause UpdateService below to fail. Reusing
			// "ctx" could make it impossible to create a service
			// if the registry is slow or unresponsive.
			var cancel func()
			ctx, cancel = c.getRequestContext()
			defer cancel()
		}

		r, err := state.controlClient.CreateService(ctx, &swarmapi.CreateServiceRequest{Spec: &serviceSpec})
		if err != nil {
			return err
		}

		resp.ID = r.Service.ID
		return nil
	})
	return resp, err
}

// UpdateService updates existing service to match new properties.
func (c *Cluster) UpdateService(serviceIDOrName string, version uint64, spec types.ServiceSpec, flags apitypes.ServiceUpdateOptions) (*apitypes.ServiceUpdateResponse, error) {
	var resp *apitypes.ServiceUpdateResponse

	err := c.lockedManagerAction(func(ctx context.Context, state nodeState) error {

		err := c.populateNetworkID(ctx, state.controlClient, &spec)
		if err != nil {
			return err
		}

		serviceSpec, err := convert.ServiceSpecToGRPC(spec)
		if err != nil {
			return apierrors.NewBadRequestError(err)
		}

		currentService, err := getService(ctx, state.controlClient, serviceIDOrName)
		if err != nil {
			return err
		}

		newCtnr := serviceSpec.Task.GetContainer()
		if newCtnr == nil {
			return errors.New("service does not use container tasks")
		}

		encodedAuth := flags.EncodedRegistryAuth
		if encodedAuth != "" {
			newCtnr.PullOptions = &swarmapi.ContainerSpec_PullOptions{RegistryAuth: encodedAuth}
		} else {
			// this is needed because if the encodedAuth isn't being updated then we
			// shouldn't lose it, and continue to use the one that was already present
			var ctnr *swarmapi.ContainerSpec
			switch flags.RegistryAuthFrom {
			case apitypes.RegistryAuthFromSpec, "":
				ctnr = currentService.Spec.Task.GetContainer()
			case apitypes.RegistryAuthFromPreviousSpec:
				if currentService.PreviousSpec == nil {
					return errors.New("service does not have a previous spec")
				}
				ctnr = currentService.PreviousSpec.Task.GetContainer()
			default:
				return errors.New("unsupported registryAuthFrom value")
			}
			if ctnr == nil {
				return errors.New("service does not use container tasks")
			}
			newCtnr.PullOptions = ctnr.PullOptions
			// update encodedAuth so it can be used to pin image by digest
			if ctnr.PullOptions != nil {
				encodedAuth = ctnr.PullOptions.RegistryAuth
			}
		}

		// retrieve auth config from encoded auth
		authConfig := &apitypes.AuthConfig{}
		if encodedAuth != "" {
			if err := json.NewDecoder(base64.NewDecoder(base64.URLEncoding, strings.NewReader(encodedAuth))).Decode(authConfig); err != nil {
				logrus.Warnf("invalid authconfig: %v", err)
			}
		}

		resp = &apitypes.ServiceUpdateResponse{}

		// pin image by digest
		if os.Getenv("DOCKER_SERVICE_PREFER_OFFLINE_IMAGE") != "1" {
			digestImage, err := c.imageWithDigestString(ctx, newCtnr.Image, authConfig)
			if err != nil {
				logrus.Warnf("unable to pin image %s to digest: %s", newCtnr.Image, err.Error())
				// warning in the client response should be concise
				resp.Warnings = append(resp.Warnings, digestWarning(newCtnr.Image))
			} else if newCtnr.Image != digestImage {
				logrus.Debugf("pinning image %s by digest: %s", newCtnr.Image, digestImage)
				newCtnr.Image = digestImage
			} else {
				logrus.Debugf("updating service using supplied digest reference %s", newCtnr.Image)
			}

			// Replace the context with a fresh one.
			// If we timed out while communicating with the
			// registry, then "ctx" will already be expired, which
			// would cause UpdateService below to fail. Reusing
			// "ctx" could make it impossible to update a service
			// if the registry is slow or unresponsive.
			var cancel func()
			ctx, cancel = c.getRequestContext()
			defer cancel()
		}

		var rollback swarmapi.UpdateServiceRequest_Rollback
		switch flags.Rollback {
		case "", "none":
			rollback = swarmapi.UpdateServiceRequest_NONE
		case "previous":
			rollback = swarmapi.UpdateServiceRequest_PREVIOUS
		default:
			return fmt.Errorf("unrecognized rollback option %s", flags.Rollback)
		}

		_, err = state.controlClient.UpdateService(
			ctx,
			&swarmapi.UpdateServiceRequest{
				ServiceID: currentService.ID,
				Spec:      &serviceSpec,
				ServiceVersion: &swarmapi.Version{
					Index: version,
				},
				Rollback: rollback,
			},
		)
		return err
	})
	return resp, err
}

// RemoveService removes a service from a managed swarm cluster.
func (c *Cluster) RemoveService(input string) error {
	return c.lockedManagerAction(func(ctx context.Context, state nodeState) error {
		service, err := getService(ctx, state.controlClient, input)
		if err != nil {
			return err
		}

		_, err = state.controlClient.RemoveService(ctx, &swarmapi.RemoveServiceRequest{ServiceID: service.ID})
		return err
	})
}

// ServiceLogs collects service logs and writes them back to `config.OutStream`
func (c *Cluster) ServiceLogs(ctx context.Context, selector *backend.LogSelector, config *backend.ContainerLogsConfig, started chan struct{}) error {
	c.mu.RLock()
	defer func() {
		select {
		case <-started:
			// if we've started streaming logs, we are no longer holding the
			// lock and do not have to release it
			return
		default:
			// before we start, though, we're holding this lock and it needs to
			// be released
			c.mu.RUnlock()
		}
	}()
	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return c.errNoManager(state)
	}

	swarmSelector, tty, err := convertSelector(ctx, state.controlClient, selector)
	if err != nil {
		return errors.Wrap(err, "error making log selector")
	}

	// TODO(dperny) this goes away when we support TTY logs, which is in the works
	if tty {
		return errors.New("service logs not supported on tasks with a TTY attached")
	}

	// set the streams we'll use
	stdStreams := []swarmapi.LogStream{}
	if config.ContainerLogsOptions.ShowStdout {
		stdStreams = append(stdStreams, swarmapi.LogStreamStdout)
	}
	if config.ContainerLogsOptions.ShowStderr {
		stdStreams = append(stdStreams, swarmapi.LogStreamStderr)
	}

	// Get tail value squared away - the number of previous log lines we look at
	var tail int64
	if config.Tail == "all" || config.Tail == "" {
		// tail of 0 means send all logs on the swarmkit side
		tail = 0
	} else {
		t, err := strconv.Atoi(config.Tail)
		if err != nil {
			return errors.New("tail value must be a positive integer or \"all\"")
		}
		if t < 0 {
			return errors.New("negative tail values not supported")
		}
		// we actually use negative tail in swarmkit to represent messages
		// backwards starting from the beginning. also, -1 means no logs. so,
		// basically, for api compat with docker container logs, add one and
		// flip the sign. we error above if you try to negative tail, which
		// isn't supported by docker (and would error deeper in the stack
		// anyway)
		//
		// See the logs protobuf for more information
		tail = int64(-(t + 1))
	}

	// get the since value - the time in the past we're looking at logs starting from
	var sinceProto *gogotypes.Timestamp
	if config.Since != "" {
		s, n, err := timetypes.ParseTimestamps(config.Since, 0)
		if err != nil {
			return errors.Wrap(err, "could not parse since timestamp")
		}
		since := time.Unix(s, n)
		sinceProto, err = gogotypes.TimestampProto(since)
		if err != nil {
			return errors.Wrap(err, "could not parse timestamp to proto")
		}
	}

	stream, err := state.logsClient.SubscribeLogs(ctx, &swarmapi.SubscribeLogsRequest{
		Selector: swarmSelector,
		Options: &swarmapi.LogSubscriptionOptions{
			Follow:  config.Follow,
			Streams: stdStreams,
			Tail:    tail,
			Since:   sinceProto,
		},
	})
	if err != nil {
		return err
	}

	wf := ioutils.NewWriteFlusher(config.OutStream)
	defer wf.Close()

	// Release the lock before starting the stream.
	//
	// this feels like it could be racy because we would double unlock if we
	// somehow returned right after we unlocked but before we closed, but I do
	// not think such a thing is possible. i wish it were possible to atomically
	// close and unlock but that might be overkill. programming is hard.
	c.mu.RUnlock()
	close(started)

	wf.Flush()

	outStream := stdcopy.NewStdWriter(wf, stdcopy.Stdout)
	errStream := stdcopy.NewStdWriter(wf, stdcopy.Stderr)

	for {
		// Check the context before doing anything.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		subscribeMsg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		for _, msg := range subscribeMsg.Messages {
			data := []byte{}

			if config.Timestamps {
				ts, err := gogotypes.TimestampFromProto(msg.Timestamp)
				if err != nil {
					return err
				}
				data = append(data, []byte(ts.Format(logger.TimeFormat)+" ")...)
			}

			data = append(data, []byte(fmt.Sprintf("%s.node.id=%s,%s.service.id=%s,%s.task.id=%s ",
				contextPrefix, msg.Context.NodeID,
				contextPrefix, msg.Context.ServiceID,
				contextPrefix, msg.Context.TaskID,
			))...)

			data = append(data, msg.Data...)

			switch msg.Stream {
			case swarmapi.LogStreamStdout:
				outStream.Write(data)
			case swarmapi.LogStreamStderr:
				errStream.Write(data)
			}
		}
	}
}

// convertSelector takes a backend.LogSelector, which contains raw names that
// may or may not be valid, and converts them to an api.LogSelector proto. It
// also returns a boolean, true if any of the services use a TTY (false
// otherwise) and an error if something fails
func convertSelector(ctx context.Context, cc swarmapi.ControlClient, selector *backend.LogSelector) (*swarmapi.LogSelector, bool, error) {
	// if ANY tasks use a TTY, don't mux streams
	var tty bool
	// don't rely on swarmkit to resolve IDs, do it ourselves
	swarmSelector := &swarmapi.LogSelector{}
	for _, s := range selector.Services {
		service, err := getService(ctx, cc, s)
		if err != nil {
			return nil, false, err
		}
		c := service.Spec.Task.GetContainer()
		if c == nil {
			return nil, false, errors.New("logs only supported on container tasks")
		}
		// set TTY true if we have a TTY service, or if it's already true
		tty = tty || c.TTY
		swarmSelector.ServiceIDs = append(swarmSelector.ServiceIDs, service.ID)
	}
	for _, t := range selector.Tasks {
		task, err := getTask(ctx, cc, t)
		if err != nil {
			return nil, false, err
		}
		c := task.Spec.GetContainer()
		if c == nil {
			return nil, false, errors.New("logs only supported on container tasks")
		}
		tty = tty || c.TTY
		swarmSelector.TaskIDs = append(swarmSelector.TaskIDs, task.ID)
	}
	return swarmSelector, tty, nil
}

// imageWithDigestString takes an image such as name or name:tag
// and returns the image pinned to a digest, such as name@sha256:34234
func (c *Cluster) imageWithDigestString(ctx context.Context, image string, authConfig *apitypes.AuthConfig) (string, error) {
	ref, err := reference.ParseAnyReference(image)
	if err != nil {
		return "", err
	}
	namedRef, ok := ref.(reference.Named)
	if !ok {
		if _, ok := ref.(reference.Digested); ok {
			return image, nil
		}
		return "", errors.Errorf("unknown image reference format: %s", image)
	}
	// only query registry if not a canonical reference (i.e. with digest)
	if _, ok := namedRef.(reference.Canonical); !ok {
		namedRef = reference.TagNameOnly(namedRef)

		taggedRef, ok := namedRef.(reference.NamedTagged)
		if !ok {
			return "", errors.Errorf("image reference not tagged: %s", image)
		}

		repo, _, err := c.config.Backend.GetRepository(ctx, taggedRef, authConfig)
		if err != nil {
			return "", err
		}
		dscrptr, err := repo.Tags(ctx).Get(ctx, taggedRef.Tag())
		if err != nil {
			return "", err
		}

		namedDigestedRef, err := reference.WithDigest(taggedRef, dscrptr.Digest)
		if err != nil {
			return "", err
		}
		// return familiar form until interface updated to return type
		return reference.FamiliarString(namedDigestedRef), nil
	}
	// reference already contains a digest, so just return it
	return reference.FamiliarString(ref), nil
}

// digestWarning constructs a formatted warning string
// using the image name that could not be pinned by digest. The
// formatting is hardcoded, but could me made smarter in the future
func digestWarning(image string) string {
	return fmt.Sprintf("image %s could not be accessed on a registry to record\nits digest. Each node will access %s independently,\npossibly leading to different nodes running different\nversions of the image.\n", image, image)
}
