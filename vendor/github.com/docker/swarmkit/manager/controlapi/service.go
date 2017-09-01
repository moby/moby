package controlapi

import (
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/defaults"
	"github.com/docker/swarmkit/api/genericresource"
	"github.com/docker/swarmkit/api/naming"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/manager/allocator"
	"github.com/docker/swarmkit/manager/constraint"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"github.com/docker/swarmkit/template"
	gogotypes "github.com/gogo/protobuf/types"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var (
	errNetworkUpdateNotSupported = errors.New("networks must be migrated to TaskSpec before being changed")
	errRenameNotSupported        = errors.New("renaming services is not supported")
	errModeChangeNotAllowed      = errors.New("service mode change is not allowed")
)

const minimumDuration = 1 * time.Millisecond

func validateResources(r *api.Resources) error {
	if r == nil {
		return nil
	}

	if r.NanoCPUs != 0 && r.NanoCPUs < 1e6 {
		return grpc.Errorf(codes.InvalidArgument, "invalid cpu value %g: Must be at least %g", float64(r.NanoCPUs)/1e9, 1e6/1e9)
	}

	if r.MemoryBytes != 0 && r.MemoryBytes < 4*1024*1024 {
		return grpc.Errorf(codes.InvalidArgument, "invalid memory value %d: Must be at least 4MiB", r.MemoryBytes)
	}
	if err := genericresource.ValidateTask(r); err != nil {
		return nil
	}
	return nil
}

func validateResourceRequirements(r *api.ResourceRequirements) error {
	if r == nil {
		return nil
	}
	if err := validateResources(r.Limits); err != nil {
		return err
	}
	if err := validateResources(r.Reservations); err != nil {
		return err
	}
	return nil
}

func validateRestartPolicy(rp *api.RestartPolicy) error {
	if rp == nil {
		return nil
	}

	if rp.Delay != nil {
		delay, err := gogotypes.DurationFromProto(rp.Delay)
		if err != nil {
			return err
		}
		if delay < 0 {
			return grpc.Errorf(codes.InvalidArgument, "TaskSpec: restart-delay cannot be negative")
		}
	}

	if rp.Window != nil {
		win, err := gogotypes.DurationFromProto(rp.Window)
		if err != nil {
			return err
		}
		if win < 0 {
			return grpc.Errorf(codes.InvalidArgument, "TaskSpec: restart-window cannot be negative")
		}
	}

	return nil
}

func validatePlacement(placement *api.Placement) error {
	if placement == nil {
		return nil
	}
	_, err := constraint.Parse(placement.Constraints)
	return err
}

func validateUpdate(uc *api.UpdateConfig) error {
	if uc == nil {
		return nil
	}

	if uc.Delay < 0 {
		return grpc.Errorf(codes.InvalidArgument, "TaskSpec: update-delay cannot be negative")
	}

	if uc.Monitor != nil {
		monitor, err := gogotypes.DurationFromProto(uc.Monitor)
		if err != nil {
			return err
		}
		if monitor < 0 {
			return grpc.Errorf(codes.InvalidArgument, "TaskSpec: update-monitor cannot be negative")
		}
	}

	if uc.MaxFailureRatio < 0 || uc.MaxFailureRatio > 1 {
		return grpc.Errorf(codes.InvalidArgument, "TaskSpec: update-maxfailureratio cannot be less than 0 or bigger than 1")
	}

	return nil
}

func validateContainerSpec(taskSpec api.TaskSpec) error {
	// Building a empty/dummy Task to validate the templating and
	// the resulting container spec as well. This is a *best effort*
	// validation.
	container, err := template.ExpandContainerSpec(&api.NodeDescription{
		Hostname: "nodeHostname",
		Platform: &api.Platform{
			OS:           "os",
			Architecture: "architecture",
		},
	}, &api.Task{
		Spec:      taskSpec,
		ServiceID: "serviceid",
		Slot:      1,
		NodeID:    "nodeid",
		Networks:  []*api.NetworkAttachment{},
		Annotations: api.Annotations{
			Name: "taskname",
		},
		ServiceAnnotations: api.Annotations{
			Name: "servicename",
		},
		Endpoint:  &api.Endpoint{},
		LogDriver: taskSpec.LogDriver,
	})
	if err != nil {
		return grpc.Errorf(codes.InvalidArgument, err.Error())
	}

	if err := validateImage(container.Image); err != nil {
		return err
	}

	if err := validateMounts(container.Mounts); err != nil {
		return err
	}

	if err := validateHealthCheck(container.Healthcheck); err != nil {
		return err
	}

	return nil
}

// validateImage validates image name in containerSpec
func validateImage(image string) error {
	if image == "" {
		return grpc.Errorf(codes.InvalidArgument, "ContainerSpec: image reference must be provided")
	}

	if _, err := reference.ParseNormalizedNamed(image); err != nil {
		return grpc.Errorf(codes.InvalidArgument, "ContainerSpec: %q is not a valid repository/tag", image)
	}
	return nil
}

// validateMounts validates if there are duplicate mounts in containerSpec
func validateMounts(mounts []api.Mount) error {
	mountMap := make(map[string]bool)
	for _, mount := range mounts {
		if _, exists := mountMap[mount.Target]; exists {
			return grpc.Errorf(codes.InvalidArgument, "ContainerSpec: duplicate mount point: %s", mount.Target)
		}
		mountMap[mount.Target] = true
	}

	return nil
}

// validateHealthCheck validates configs about container's health check
func validateHealthCheck(hc *api.HealthConfig) error {
	if hc == nil {
		return nil
	}

	if hc.Interval != nil {
		interval, err := gogotypes.DurationFromProto(hc.Interval)
		if err != nil {
			return err
		}
		if interval != 0 && interval < time.Duration(minimumDuration) {
			return grpc.Errorf(codes.InvalidArgument, "ContainerSpec: Interval in HealthConfig cannot be less than %s", minimumDuration)
		}
	}

	if hc.Timeout != nil {
		timeout, err := gogotypes.DurationFromProto(hc.Timeout)
		if err != nil {
			return err
		}
		if timeout != 0 && timeout < time.Duration(minimumDuration) {
			return grpc.Errorf(codes.InvalidArgument, "ContainerSpec: Timeout in HealthConfig cannot be less than %s", minimumDuration)
		}
	}

	if hc.StartPeriod != nil {
		sp, err := gogotypes.DurationFromProto(hc.StartPeriod)
		if err != nil {
			return err
		}
		if sp != 0 && sp < time.Duration(minimumDuration) {
			return grpc.Errorf(codes.InvalidArgument, "ContainerSpec: StartPeriod in HealthConfig cannot be less than %s", minimumDuration)
		}
	}

	if hc.Retries < 0 {
		return grpc.Errorf(codes.InvalidArgument, "ContainerSpec: Retries in HealthConfig cannot be negative")
	}

	return nil
}

func validateGenericRuntimeSpec(taskSpec api.TaskSpec) error {
	generic := taskSpec.GetGeneric()

	if len(generic.Kind) < 3 {
		return grpc.Errorf(codes.InvalidArgument, "Generic runtime: Invalid name %q", generic.Kind)
	}

	reservedNames := []string{"container", "attachment"}
	for _, n := range reservedNames {
		if strings.ToLower(generic.Kind) == n {
			return grpc.Errorf(codes.InvalidArgument, "Generic runtime: %q is a reserved name", generic.Kind)
		}
	}

	payload := generic.Payload

	if payload == nil {
		return grpc.Errorf(codes.InvalidArgument, "Generic runtime is missing payload")
	}

	if payload.TypeUrl == "" {
		return grpc.Errorf(codes.InvalidArgument, "Generic runtime is missing payload type")
	}

	if len(payload.Value) == 0 {
		return grpc.Errorf(codes.InvalidArgument, "Generic runtime has an empty payload")
	}

	return nil
}

func validateTaskSpec(taskSpec api.TaskSpec) error {
	if err := validateResourceRequirements(taskSpec.Resources); err != nil {
		return err
	}

	if err := validateRestartPolicy(taskSpec.Restart); err != nil {
		return err
	}

	if err := validatePlacement(taskSpec.Placement); err != nil {
		return err
	}

	// Check to see if the secret reference portion of the spec is valid
	if err := validateSecretRefsSpec(taskSpec); err != nil {
		return err
	}

	// Check to see if the config reference portion of the spec is valid
	if err := validateConfigRefsSpec(taskSpec); err != nil {
		return err
	}

	if taskSpec.GetRuntime() == nil {
		return grpc.Errorf(codes.InvalidArgument, "TaskSpec: missing runtime")
	}

	switch taskSpec.GetRuntime().(type) {
	case *api.TaskSpec_Container:
		if err := validateContainerSpec(taskSpec); err != nil {
			return err
		}
	case *api.TaskSpec_Generic:
		if err := validateGenericRuntimeSpec(taskSpec); err != nil {
			return err
		}
	default:
		return grpc.Errorf(codes.Unimplemented, "RuntimeSpec: unimplemented runtime in service spec")
	}

	return nil
}

func validateEndpointSpec(epSpec *api.EndpointSpec) error {
	// Endpoint spec is optional
	if epSpec == nil {
		return nil
	}

	type portSpec struct {
		publishedPort uint32
		protocol      api.PortConfig_Protocol
	}

	portSet := make(map[portSpec]struct{})
	for _, port := range epSpec.Ports {
		// Publish mode = "ingress" represents Routing-Mesh and current implementation
		// of routing-mesh relies on IPVS based load-balancing with input=published-port.
		// But Endpoint-Spec mode of DNSRR relies on multiple A records and cannot be used
		// with routing-mesh (PublishMode="ingress") which cannot rely on DNSRR.
		// But PublishMode="host" doesn't provide Routing-Mesh and the DNSRR is applicable
		// for the backend network and hence we accept that configuration.

		if epSpec.Mode == api.ResolutionModeDNSRoundRobin && port.PublishMode == api.PublishModeIngress {
			return grpc.Errorf(codes.InvalidArgument, "EndpointSpec: port published with ingress mode can't be used with dnsrr mode")
		}

		// If published port is not specified, it does not conflict
		// with any others.
		if port.PublishedPort == 0 {
			continue
		}

		portSpec := portSpec{publishedPort: port.PublishedPort, protocol: port.Protocol}
		if _, ok := portSet[portSpec]; ok {
			return grpc.Errorf(codes.InvalidArgument, "EndpointSpec: duplicate published ports provided")
		}

		portSet[portSpec] = struct{}{}
	}

	return nil
}

// validateSecretRefsSpec finds if the secrets passed in spec are valid and have no
// conflicting targets.
func validateSecretRefsSpec(spec api.TaskSpec) error {
	container := spec.GetContainer()
	if container == nil {
		return nil
	}

	// Keep a map to track all the targets that will be exposed
	// The string returned is only used for logging. It could as well be struct{}{}
	existingTargets := make(map[string]string)
	for _, secretRef := range container.Secrets {
		// SecretID and SecretName are mandatory, we have invalid references without them
		if secretRef.SecretID == "" || secretRef.SecretName == "" {
			return grpc.Errorf(codes.InvalidArgument, "malformed secret reference")
		}

		// Every secret reference requires a Target
		if secretRef.GetTarget() == nil {
			return grpc.Errorf(codes.InvalidArgument, "malformed secret reference, no target provided")
		}

		// If this is a file target, we will ensure filename uniqueness
		if secretRef.GetFile() != nil {
			fileName := secretRef.GetFile().Name
			if fileName == "" {
				return grpc.Errorf(codes.InvalidArgument, "malformed file secret reference, invalid target file name provided")
			}
			// If this target is already in use, we have conflicting targets
			if prevSecretName, ok := existingTargets[fileName]; ok {
				return grpc.Errorf(codes.InvalidArgument, "secret references '%s' and '%s' have a conflicting target: '%s'", prevSecretName, secretRef.SecretName, fileName)
			}

			existingTargets[fileName] = secretRef.SecretName
		}
	}

	return nil
}

// validateConfigRefsSpec finds if the configs passed in spec are valid and have no
// conflicting targets.
func validateConfigRefsSpec(spec api.TaskSpec) error {
	container := spec.GetContainer()
	if container == nil {
		return nil
	}

	// Keep a map to track all the targets that will be exposed
	// The string returned is only used for logging. It could as well be struct{}{}
	existingTargets := make(map[string]string)
	for _, configRef := range container.Configs {
		// ConfigID and ConfigName are mandatory, we have invalid references without them
		if configRef.ConfigID == "" || configRef.ConfigName == "" {
			return grpc.Errorf(codes.InvalidArgument, "malformed config reference")
		}

		// Every config reference requires a Target
		if configRef.GetTarget() == nil {
			return grpc.Errorf(codes.InvalidArgument, "malformed config reference, no target provided")
		}

		// If this is a file target, we will ensure filename uniqueness
		if configRef.GetFile() != nil {
			fileName := configRef.GetFile().Name
			// Validate the file name
			if fileName == "" {
				return grpc.Errorf(codes.InvalidArgument, "malformed file config reference, invalid target file name provided")
			}

			// If this target is already in use, we have conflicting targets
			if prevConfigName, ok := existingTargets[fileName]; ok {
				return grpc.Errorf(codes.InvalidArgument, "config references '%s' and '%s' have a conflicting target: '%s'", prevConfigName, configRef.ConfigName, fileName)
			}

			existingTargets[fileName] = configRef.ConfigName
		}
	}

	return nil
}

func (s *Server) validateNetworks(networks []*api.NetworkAttachmentConfig) error {
	for _, na := range networks {
		var network *api.Network
		s.store.View(func(tx store.ReadTx) {
			network = store.GetNetwork(tx, na.Target)
		})
		if network == nil {
			continue
		}
		if allocator.IsIngressNetwork(network) {
			return grpc.Errorf(codes.InvalidArgument,
				"Service cannot be explicitly attached to the ingress network %q", network.Spec.Annotations.Name)
		}
	}
	return nil
}

func validateMode(s *api.ServiceSpec) error {
	m := s.GetMode()
	switch m.(type) {
	case *api.ServiceSpec_Replicated:
		if int64(m.(*api.ServiceSpec_Replicated).Replicated.Replicas) < 0 {
			return grpc.Errorf(codes.InvalidArgument, "Number of replicas must be non-negative")
		}
	case *api.ServiceSpec_Global:
	default:
		return grpc.Errorf(codes.InvalidArgument, "Unrecognized service mode")
	}

	return nil
}

func validateServiceSpec(spec *api.ServiceSpec) error {
	if spec == nil {
		return grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}
	if err := validateAnnotations(spec.Annotations); err != nil {
		return err
	}
	if err := validateTaskSpec(spec.Task); err != nil {
		return err
	}
	if err := validateUpdate(spec.Update); err != nil {
		return err
	}
	if err := validateEndpointSpec(spec.Endpoint); err != nil {
		return err
	}
	if err := validateMode(spec); err != nil {
		return err
	}

	return nil
}

// checkPortConflicts does a best effort to find if the passed in spec has port
// conflicts with existing services.
// `serviceID string` is the service ID of the spec in service update. If
// `serviceID` is not "", then conflicts check will be skipped against this
// service (the service being updated).
func (s *Server) checkPortConflicts(spec *api.ServiceSpec, serviceID string) error {
	if spec.Endpoint == nil {
		return nil
	}

	type portSpec struct {
		protocol      api.PortConfig_Protocol
		publishedPort uint32
	}

	pcToStruct := func(pc *api.PortConfig) portSpec {
		return portSpec{
			protocol:      pc.Protocol,
			publishedPort: pc.PublishedPort,
		}
	}

	ingressPorts := make(map[portSpec]struct{})
	hostModePorts := make(map[portSpec]struct{})
	for _, pc := range spec.Endpoint.Ports {
		if pc.PublishedPort == 0 {
			continue
		}
		switch pc.PublishMode {
		case api.PublishModeIngress:
			ingressPorts[pcToStruct(pc)] = struct{}{}
		case api.PublishModeHost:
			hostModePorts[pcToStruct(pc)] = struct{}{}
		}
	}
	if len(ingressPorts) == 0 && len(hostModePorts) == 0 {
		return nil
	}

	var (
		services []*api.Service
		err      error
	)

	s.store.View(func(tx store.ReadTx) {
		services, err = store.FindServices(tx, store.All)
	})
	if err != nil {
		return err
	}

	isPortInUse := func(pc *api.PortConfig, service *api.Service) error {
		if pc.PublishedPort == 0 {
			return nil
		}

		switch pc.PublishMode {
		case api.PublishModeHost:
			if _, ok := ingressPorts[pcToStruct(pc)]; ok {
				return grpc.Errorf(codes.InvalidArgument, "port '%d' is already in use by service '%s' (%s) as a host-published port", pc.PublishedPort, service.Spec.Annotations.Name, service.ID)
			}

			// Multiple services with same port in host publish mode can
			// coexist - this is handled by the scheduler.
			return nil
		case api.PublishModeIngress:
			_, ingressConflict := ingressPorts[pcToStruct(pc)]
			_, hostModeConflict := hostModePorts[pcToStruct(pc)]
			if ingressConflict || hostModeConflict {
				return grpc.Errorf(codes.InvalidArgument, "port '%d' is already in use by service '%s' (%s) as an ingress port", pc.PublishedPort, service.Spec.Annotations.Name, service.ID)
			}
		}

		return nil
	}

	for _, service := range services {
		// If service ID is the same (and not "") then this is an update
		if serviceID != "" && serviceID == service.ID {
			continue
		}
		if service.Spec.Endpoint != nil {
			for _, pc := range service.Spec.Endpoint.Ports {
				if err := isPortInUse(pc, service); err != nil {
					return err
				}
			}
		}
		if service.Endpoint != nil {
			for _, pc := range service.Endpoint.Ports {
				if err := isPortInUse(pc, service); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// checkSecretExistence finds if the secret exists
func (s *Server) checkSecretExistence(tx store.Tx, spec *api.ServiceSpec) error {
	container := spec.Task.GetContainer()
	if container == nil {
		return nil
	}

	var failedSecrets []string
	for _, secretRef := range container.Secrets {
		secret := store.GetSecret(tx, secretRef.SecretID)
		// Check to see if the secret exists and secretRef.SecretName matches the actual secretName
		if secret == nil || secret.Spec.Annotations.Name != secretRef.SecretName {
			failedSecrets = append(failedSecrets, secretRef.SecretName)
		}
	}

	if len(failedSecrets) > 0 {
		secretStr := "secrets"
		if len(failedSecrets) == 1 {
			secretStr = "secret"
		}

		return grpc.Errorf(codes.InvalidArgument, "%s not found: %v", secretStr, strings.Join(failedSecrets, ", "))

	}

	return nil
}

// checkConfigExistence finds if the config exists
func (s *Server) checkConfigExistence(tx store.Tx, spec *api.ServiceSpec) error {
	container := spec.Task.GetContainer()
	if container == nil {
		return nil
	}

	var failedConfigs []string
	for _, configRef := range container.Configs {
		config := store.GetConfig(tx, configRef.ConfigID)
		// Check to see if the config exists and configRef.ConfigName matches the actual configName
		if config == nil || config.Spec.Annotations.Name != configRef.ConfigName {
			failedConfigs = append(failedConfigs, configRef.ConfigName)
		}
	}

	if len(failedConfigs) > 0 {
		configStr := "configs"
		if len(failedConfigs) == 1 {
			configStr = "config"
		}

		return grpc.Errorf(codes.InvalidArgument, "%s not found: %v", configStr, strings.Join(failedConfigs, ", "))

	}

	return nil
}

// CreateService creates and returns a Service based on the provided ServiceSpec.
// - Returns `InvalidArgument` if the ServiceSpec is malformed.
// - Returns `Unimplemented` if the ServiceSpec references unimplemented features.
// - Returns `AlreadyExists` if the ServiceID conflicts.
// - Returns an error if the creation fails.
func (s *Server) CreateService(ctx context.Context, request *api.CreateServiceRequest) (*api.CreateServiceResponse, error) {
	if err := validateServiceSpec(request.Spec); err != nil {
		return nil, err
	}

	if err := s.validateNetworks(request.Spec.Networks); err != nil {
		return nil, err
	}

	if err := s.checkPortConflicts(request.Spec, ""); err != nil {
		return nil, err
	}

	// TODO(aluzzardi): Consider using `Name` as a primary key to handle
	// duplicate creations. See #65
	service := &api.Service{
		ID:          identity.NewID(),
		Spec:        *request.Spec,
		SpecVersion: &api.Version{},
	}

	if allocator.IsIngressNetworkNeeded(service) {
		if _, err := allocator.GetIngressNetwork(s.store); err == allocator.ErrNoIngress {
			return nil, grpc.Errorf(codes.FailedPrecondition, "service needs ingress network, but no ingress network is present")
		}
	}

	err := s.store.Update(func(tx store.Tx) error {
		// Check to see if all the secrets being added exist as objects
		// in our datastore
		err := s.checkSecretExistence(tx, request.Spec)
		if err != nil {
			return err
		}
		err = s.checkConfigExistence(tx, request.Spec)
		if err != nil {
			return err
		}

		return store.CreateService(tx, service)
	})
	if err != nil {
		return nil, err
	}

	return &api.CreateServiceResponse{
		Service: service,
	}, nil
}

// GetService returns a Service given a ServiceID.
// - Returns `InvalidArgument` if ServiceID is not provided.
// - Returns `NotFound` if the Service is not found.
func (s *Server) GetService(ctx context.Context, request *api.GetServiceRequest) (*api.GetServiceResponse, error) {
	if request.ServiceID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	var service *api.Service
	s.store.View(func(tx store.ReadTx) {
		service = store.GetService(tx, request.ServiceID)
	})
	if service == nil {
		return nil, grpc.Errorf(codes.NotFound, "service %s not found", request.ServiceID)
	}

	if request.InsertDefaults {
		service.Spec = *defaults.InterpolateService(&service.Spec)
	}

	return &api.GetServiceResponse{
		Service: service,
	}, nil
}

// UpdateService updates a Service referenced by ServiceID with the given ServiceSpec.
// - Returns `NotFound` if the Service is not found.
// - Returns `InvalidArgument` if the ServiceSpec is malformed.
// - Returns `Unimplemented` if the ServiceSpec references unimplemented features.
// - Returns an error if the update fails.
func (s *Server) UpdateService(ctx context.Context, request *api.UpdateServiceRequest) (*api.UpdateServiceResponse, error) {
	if request.ServiceID == "" || request.ServiceVersion == nil {
		return nil, grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}
	if err := validateServiceSpec(request.Spec); err != nil {
		return nil, err
	}

	var service *api.Service
	s.store.View(func(tx store.ReadTx) {
		service = store.GetService(tx, request.ServiceID)
	})
	if service == nil {
		return nil, grpc.Errorf(codes.NotFound, "service %s not found", request.ServiceID)
	}

	if request.Spec.Endpoint != nil && !reflect.DeepEqual(request.Spec.Endpoint, service.Spec.Endpoint) {
		if err := s.checkPortConflicts(request.Spec, request.ServiceID); err != nil {
			return nil, err
		}
	}

	err := s.store.Update(func(tx store.Tx) error {
		service = store.GetService(tx, request.ServiceID)
		if service == nil {
			return grpc.Errorf(codes.NotFound, "service %s not found", request.ServiceID)
		}

		// It's not okay to update Service.Spec.Networks on its own.
		// However, if Service.Spec.Task.Networks is also being
		// updated, that's okay (for example when migrating from the
		// deprecated Spec.Networks field to Spec.Task.Networks).
		if (len(request.Spec.Networks) != 0 || len(service.Spec.Networks) != 0) &&
			!reflect.DeepEqual(request.Spec.Networks, service.Spec.Networks) &&
			reflect.DeepEqual(request.Spec.Task.Networks, service.Spec.Task.Networks) {
			return grpc.Errorf(codes.Unimplemented, errNetworkUpdateNotSupported.Error())
		}

		// Check to see if all the secrets being added exist as objects
		// in our datastore
		err := s.checkSecretExistence(tx, request.Spec)
		if err != nil {
			return err
		}

		err = s.checkConfigExistence(tx, request.Spec)
		if err != nil {
			return err
		}

		// orchestrator is designed to be stateless, so it should not deal
		// with service mode change (comparing current config with previous config).
		// proper way to change service mode is to delete and re-add.
		if reflect.TypeOf(service.Spec.Mode) != reflect.TypeOf(request.Spec.Mode) {
			return grpc.Errorf(codes.Unimplemented, errModeChangeNotAllowed.Error())
		}

		if service.Spec.Annotations.Name != request.Spec.Annotations.Name {
			return grpc.Errorf(codes.Unimplemented, errRenameNotSupported.Error())
		}

		service.Meta.Version = *request.ServiceVersion

		if request.Rollback == api.UpdateServiceRequest_PREVIOUS {
			if service.PreviousSpec == nil {
				return grpc.Errorf(codes.FailedPrecondition, "service %s does not have a previous spec", request.ServiceID)
			}

			curSpec := service.Spec.Copy()
			curSpecVersion := service.SpecVersion
			service.Spec = *service.PreviousSpec.Copy()
			service.SpecVersion = service.PreviousSpecVersion.Copy()
			service.PreviousSpec = curSpec
			service.PreviousSpecVersion = curSpecVersion

			service.UpdateStatus = &api.UpdateStatus{
				State:     api.UpdateStatus_ROLLBACK_STARTED,
				Message:   "manually requested rollback",
				StartedAt: ptypes.MustTimestampProto(time.Now()),
			}
		} else {
			service.PreviousSpec = service.Spec.Copy()
			service.PreviousSpecVersion = service.SpecVersion
			service.Spec = *request.Spec.Copy()
			// Set spec version. Note that this will not match the
			// service's Meta.Version after the store update. The
			// versions for the spec and the service itself are not
			// meant to be directly comparable.
			service.SpecVersion = service.Meta.Version.Copy()

			// Reset update status
			service.UpdateStatus = nil
		}

		if allocator.IsIngressNetworkNeeded(service) {
			if _, err := allocator.GetIngressNetwork(s.store); err == allocator.ErrNoIngress {
				return grpc.Errorf(codes.FailedPrecondition, "service needs ingress network, but no ingress network is present")
			}
		}

		return store.UpdateService(tx, service)
	})
	if err != nil {
		return nil, err
	}

	return &api.UpdateServiceResponse{
		Service: service,
	}, nil
}

// RemoveService removes a Service referenced by ServiceID.
// - Returns `InvalidArgument` if ServiceID is not provided.
// - Returns `NotFound` if the Service is not found.
// - Returns an error if the deletion fails.
func (s *Server) RemoveService(ctx context.Context, request *api.RemoveServiceRequest) (*api.RemoveServiceResponse, error) {
	if request.ServiceID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	err := s.store.Update(func(tx store.Tx) error {
		return store.DeleteService(tx, request.ServiceID)
	})
	if err != nil {
		if err == store.ErrNotExist {
			return nil, grpc.Errorf(codes.NotFound, "service %s not found", request.ServiceID)
		}
		return nil, err
	}
	return &api.RemoveServiceResponse{}, nil
}

func filterServices(candidates []*api.Service, filters ...func(*api.Service) bool) []*api.Service {
	result := []*api.Service{}

	for _, c := range candidates {
		match := true
		for _, f := range filters {
			if !f(c) {
				match = false
				break
			}
		}
		if match {
			result = append(result, c)
		}
	}

	return result
}

// ListServices returns a list of all services.
func (s *Server) ListServices(ctx context.Context, request *api.ListServicesRequest) (*api.ListServicesResponse, error) {
	var (
		services []*api.Service
		err      error
	)

	s.store.View(func(tx store.ReadTx) {
		switch {
		case request.Filters != nil && len(request.Filters.Names) > 0:
			services, err = store.FindServices(tx, buildFilters(store.ByName, request.Filters.Names))
		case request.Filters != nil && len(request.Filters.NamePrefixes) > 0:
			services, err = store.FindServices(tx, buildFilters(store.ByNamePrefix, request.Filters.NamePrefixes))
		case request.Filters != nil && len(request.Filters.IDPrefixes) > 0:
			services, err = store.FindServices(tx, buildFilters(store.ByIDPrefix, request.Filters.IDPrefixes))
		case request.Filters != nil && len(request.Filters.Runtimes) > 0:
			services, err = store.FindServices(tx, buildFilters(store.ByRuntime, request.Filters.Runtimes))
		default:
			services, err = store.FindServices(tx, store.All)
		}
	})
	if err != nil {
		return nil, err
	}

	if request.Filters != nil {
		services = filterServices(services,
			func(e *api.Service) bool {
				return filterContains(e.Spec.Annotations.Name, request.Filters.Names)
			},
			func(e *api.Service) bool {
				return filterContainsPrefix(e.Spec.Annotations.Name, request.Filters.NamePrefixes)
			},
			func(e *api.Service) bool {
				return filterContainsPrefix(e.ID, request.Filters.IDPrefixes)
			},
			func(e *api.Service) bool {
				return filterMatchLabels(e.Spec.Annotations.Labels, request.Filters.Labels)
			},
			func(e *api.Service) bool {
				if len(request.Filters.Runtimes) == 0 {
					return true
				}
				r, err := naming.Runtime(e.Spec.Task)
				if err != nil {
					return false
				}
				return filterContains(r, request.Filters.Runtimes)
			},
		)
	}

	return &api.ListServicesResponse{
		Services: services,
	}, nil
}
