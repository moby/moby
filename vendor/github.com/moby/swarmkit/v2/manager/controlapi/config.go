package controlapi

import (
	"bytes"
	"context"
	"strings"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/identity"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MaxConfigSize is the maximum byte length of the `Config.Spec.Data` field.
const MaxConfigSize = 1000 * 1024 // 1000KB

// assumes spec is not nil
func configFromConfigSpec(spec *api.ConfigSpec) *api.Config {
	return &api.Config{
		ID:   identity.NewID(),
		Spec: *spec,
	}
}

// GetConfig returns a `GetConfigResponse` with a `Config` with the same
// id as `GetConfigRequest.ConfigID`
// - Returns `NotFound` if the Config with the given id is not found.
// - Returns `InvalidArgument` if the `GetConfigRequest.ConfigID` is empty.
// - Returns an error if getting fails.
func (s *Server) GetConfig(ctx context.Context, request *api.GetConfigRequest) (*api.GetConfigResponse, error) {
	if request.ConfigID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "config ID must be provided")
	}

	var config *api.Config
	s.store.View(func(tx store.ReadTx) {
		config = store.GetConfig(tx, request.ConfigID)
	})

	if config == nil {
		return nil, status.Errorf(codes.NotFound, "config %s not found", request.ConfigID)
	}

	return &api.GetConfigResponse{Config: config}, nil
}

// UpdateConfig updates a Config referenced by ConfigID with the given ConfigSpec.
// - Returns `NotFound` if the Config is not found.
// - Returns `InvalidArgument` if the ConfigSpec is malformed or anything other than Labels is changed
// - Returns an error if the update fails.
func (s *Server) UpdateConfig(ctx context.Context, request *api.UpdateConfigRequest) (*api.UpdateConfigResponse, error) {
	if request.ConfigID == "" || request.ConfigVersion == nil {
		return nil, status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	var config *api.Config
	err := s.store.Update(func(tx store.Tx) error {
		config = store.GetConfig(tx, request.ConfigID)
		if config == nil {
			return status.Errorf(codes.NotFound, "config %s not found", request.ConfigID)
		}

		// Check if the Name is different than the current name, or the config is non-nil and different
		// than the current config
		if config.Spec.Annotations.Name != request.Spec.Annotations.Name ||
			(request.Spec.Data != nil && !bytes.Equal(request.Spec.Data, config.Spec.Data)) {
			return status.Errorf(codes.InvalidArgument, "only updates to Labels are allowed")
		}

		// We only allow updating Labels
		config.Meta.Version = *request.ConfigVersion
		config.Spec.Annotations.Labels = request.Spec.Annotations.Labels

		return store.UpdateConfig(tx, config)
	})
	if err != nil {
		return nil, err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"config.ID":   request.ConfigID,
		"config.Name": request.Spec.Annotations.Name,
		"method":      "UpdateConfig",
	}).Debugf("config updated")

	return &api.UpdateConfigResponse{
		Config: config,
	}, nil
}

// ListConfigs returns a `ListConfigResponse` with a list all non-internal `Config`s being
// managed, or all configs matching any name in `ListConfigsRequest.Names`, any
// name prefix in `ListConfigsRequest.NamePrefixes`, any id in
// `ListConfigsRequest.ConfigIDs`, or any id prefix in `ListConfigsRequest.IDPrefixes`.
// - Returns an error if listing fails.
func (s *Server) ListConfigs(ctx context.Context, request *api.ListConfigsRequest) (*api.ListConfigsResponse, error) {
	var (
		configs     []*api.Config
		respConfigs []*api.Config
		err         error
		byFilters   []store.By
		by          store.By
		labels      map[string]string
	)

	// return all configs that match either any of the names or any of the name prefixes (why would you give both?)
	if request.Filters != nil {
		for _, name := range request.Filters.Names {
			byFilters = append(byFilters, store.ByName(name))
		}
		for _, prefix := range request.Filters.NamePrefixes {
			byFilters = append(byFilters, store.ByNamePrefix(prefix))
		}
		for _, prefix := range request.Filters.IDPrefixes {
			byFilters = append(byFilters, store.ByIDPrefix(prefix))
		}
		labels = request.Filters.Labels
	}

	switch len(byFilters) {
	case 0:
		by = store.All
	case 1:
		by = byFilters[0]
	default:
		by = store.Or(byFilters...)
	}

	s.store.View(func(tx store.ReadTx) {
		configs, err = store.FindConfigs(tx, by)
	})
	if err != nil {
		return nil, err
	}

	// filter by label
	for _, config := range configs {
		if !filterMatchLabels(config.Spec.Annotations.Labels, labels) {
			continue
		}
		respConfigs = append(respConfigs, config)
	}

	return &api.ListConfigsResponse{Configs: respConfigs}, nil
}

// CreateConfig creates and returns a `CreateConfigResponse` with a `Config` based
// on the provided `CreateConfigRequest.ConfigSpec`.
// - Returns `InvalidArgument` if the `CreateConfigRequest.ConfigSpec` is malformed,
//   or if the config data is too long or contains invalid characters.
// - Returns an error if the creation fails.
func (s *Server) CreateConfig(ctx context.Context, request *api.CreateConfigRequest) (*api.CreateConfigResponse, error) {
	if err := validateConfigSpec(request.Spec); err != nil {
		return nil, err
	}

	config := configFromConfigSpec(request.Spec) // the store will handle name conflicts
	err := s.store.Update(func(tx store.Tx) error {
		return store.CreateConfig(tx, config)
	})

	switch err {
	case store.ErrNameConflict:
		return nil, status.Errorf(codes.AlreadyExists, "config %s already exists", request.Spec.Annotations.Name)
	case nil:
		log.G(ctx).WithFields(logrus.Fields{
			"config.Name": request.Spec.Annotations.Name,
			"method":      "CreateConfig",
		}).Debugf("config created")

		return &api.CreateConfigResponse{Config: config}, nil
	default:
		return nil, err
	}
}

// RemoveConfig removes the config referenced by `RemoveConfigRequest.ID`.
// - Returns `InvalidArgument` if `RemoveConfigRequest.ID` is empty.
// - Returns `NotFound` if the a config named `RemoveConfigRequest.ID` is not found.
// - Returns `ConfigInUse` if the config is currently in use
// - Returns an error if the deletion fails.
func (s *Server) RemoveConfig(ctx context.Context, request *api.RemoveConfigRequest) (*api.RemoveConfigResponse, error) {
	if request.ConfigID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "config ID must be provided")
	}

	err := s.store.Update(func(tx store.Tx) error {
		// Check if the config exists
		config := store.GetConfig(tx, request.ConfigID)
		if config == nil {
			return status.Errorf(codes.NotFound, "could not find config %s", request.ConfigID)
		}

		// Check if any services currently reference this config, return error if so
		services, err := store.FindServices(tx, store.ByReferencedConfigID(request.ConfigID))
		if err != nil {
			return status.Errorf(codes.Internal, "could not find services using config %s: %v", request.ConfigID, err)
		}

		if len(services) != 0 {
			serviceNames := make([]string, 0, len(services))
			for _, service := range services {
				serviceNames = append(serviceNames, service.Spec.Annotations.Name)
			}

			configName := config.Spec.Annotations.Name
			serviceNameStr := strings.Join(serviceNames, ", ")
			serviceStr := "services"
			if len(serviceNames) == 1 {
				serviceStr = "service"
			}

			return status.Errorf(codes.InvalidArgument, "config '%s' is in use by the following %s: %v", configName, serviceStr, serviceNameStr)
		}

		return store.DeleteConfig(tx, request.ConfigID)
	})
	switch err {
	case store.ErrNotExist:
		return nil, status.Errorf(codes.NotFound, "config %s not found", request.ConfigID)
	case nil:
		log.G(ctx).WithFields(logrus.Fields{
			"config.ID": request.ConfigID,
			"method":    "RemoveConfig",
		}).Debugf("config removed")

		return &api.RemoveConfigResponse{}, nil
	default:
		return nil, err
	}
}

func validateConfigSpec(spec *api.ConfigSpec) error {
	if spec == nil {
		return status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}
	if err := validateConfigOrSecretAnnotations(spec.Annotations); err != nil {
		return err
	}

	if len(spec.Data) >= MaxConfigSize || len(spec.Data) < 1 {
		return status.Errorf(codes.InvalidArgument, "config data must be larger than 0 and less than %d bytes", MaxConfigSize)
	}
	return nil
}
