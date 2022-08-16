package controlapi

import (
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

// CreateExtension creates an `Extension` based on the provided `CreateExtensionRequest.Extension`
// and returns a `CreateExtensionResponse`.
//   - Returns `InvalidArgument` if the `CreateExtensionRequest.Extension` is malformed,
//     or fails validation.
//   - Returns an error if the creation fails.
func (s *Server) CreateExtension(ctx context.Context, request *api.CreateExtensionRequest) (*api.CreateExtensionResponse, error) {
	if request.Annotations == nil || request.Annotations.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "extension name must be provided")
	}

	extension := &api.Extension{
		ID:          identity.NewID(),
		Annotations: *request.Annotations,
		Description: request.Description,
	}

	err := s.store.Update(func(tx store.Tx) error {
		return store.CreateExtension(tx, extension)
	})

	switch err {
	case store.ErrNameConflict:
		return nil, status.Errorf(codes.AlreadyExists, "extension %s already exists", request.Annotations.Name)
	case nil:
		log.G(ctx).WithFields(logrus.Fields{
			"extension.Name": request.Annotations.Name,
			"method":         "CreateExtension",
		}).Debugf("extension created")

		return &api.CreateExtensionResponse{Extension: extension}, nil
	default:
		return nil, status.Errorf(codes.Internal, "could not create extension: %v", err.Error())
	}
}

// GetExtension returns a `GetExtensionResponse` with a `Extension` with the same
// id as `GetExtensionRequest.extension_id`
// - Returns `NotFound` if the Extension with the given id is not found.
// - Returns `InvalidArgument` if the `GetExtensionRequest.extension_id` is empty.
// - Returns an error if the get fails.
func (s *Server) GetExtension(ctx context.Context, request *api.GetExtensionRequest) (*api.GetExtensionResponse, error) {
	if request.ExtensionID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "extension ID must be provided")
	}

	var extension *api.Extension
	s.store.View(func(tx store.ReadTx) {
		extension = store.GetExtension(tx, request.ExtensionID)
	})

	if extension == nil {
		return nil, status.Errorf(codes.NotFound, "extension %s not found", request.ExtensionID)
	}

	return &api.GetExtensionResponse{Extension: extension}, nil
}

// RemoveExtension removes the extension referenced by `RemoveExtensionRequest.ID`.
// - Returns `InvalidArgument` if `RemoveExtensionRequest.extension_id` is empty.
// - Returns `NotFound` if the an extension named `RemoveExtensionRequest.extension_id` is not found.
// - Returns an error if the deletion fails.
func (s *Server) RemoveExtension(ctx context.Context, request *api.RemoveExtensionRequest) (*api.RemoveExtensionResponse, error) {
	if request.ExtensionID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "extension ID must be provided")
	}

	err := s.store.Update(func(tx store.Tx) error {
		// Check if the extension exists
		extension := store.GetExtension(tx, request.ExtensionID)
		if extension == nil {
			return status.Errorf(codes.NotFound, "could not find extension %s", request.ExtensionID)
		}

		// Check if any resources of this type present in the store, return error if so
		resources, err := store.FindResources(tx, store.ByKind(request.ExtensionID))
		if err != nil {
			return status.Errorf(codes.Internal, "could not find resources using extension %s: %v", request.ExtensionID, err)
		}

		if len(resources) != 0 {
			resourceNames := make([]string, 0, len(resources))
			// Number of resources for an extension could be quite large.
			// Show a limited number of resources for debugging.
			attachedResourceForDebug := 10
			for _, resource := range resources {
				resourceNames = append(resourceNames, resource.Annotations.Name)
				attachedResourceForDebug = attachedResourceForDebug - 1
				if attachedResourceForDebug == 0 {
					break
				}
			}

			extensionName := extension.Annotations.Name
			resourceNameStr := strings.Join(resourceNames, ", ")
			resourceStr := "resources"
			if len(resourceNames) == 1 {
				resourceStr = "resource"
			}

			return status.Errorf(codes.InvalidArgument, "extension '%s' is in use by the following %s: %v", extensionName, resourceStr, resourceNameStr)
		}

		return store.DeleteExtension(tx, request.ExtensionID)
	})
	switch err {
	case store.ErrNotExist:
		return nil, status.Errorf(codes.NotFound, "extension %s not found", request.ExtensionID)
	case nil:
		log.G(ctx).WithFields(logrus.Fields{
			"extension.ID": request.ExtensionID,
			"method":       "RemoveExtension",
		}).Debugf("extension removed")

		return &api.RemoveExtensionResponse{}, nil
	default:
		return nil, err
	}
}
