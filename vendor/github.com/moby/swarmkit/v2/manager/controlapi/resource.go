package controlapi

import (
	"context"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/identity"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/state/store"
)

// CreateResource returns a `CreateResourceResponse` after creating a `Resource` based
// on the provided `CreateResourceRequest.Resource`.
// - Returns `InvalidArgument` if the `CreateResourceRequest.Resource` is malformed,
//   or if the config data is too long or contains invalid characters.
// - Returns an error if the creation fails.
func (s *Server) CreateResource(ctx context.Context, request *api.CreateResourceRequest) (*api.CreateResourceResponse, error) {
	if request.Annotations == nil || request.Annotations.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Resource must have a name")
	}

	// finally, validate that Kind is not an emptystring. We know that creating
	// with Kind as empty string should fail at the store level, but to make
	// errors clearer, special case this.
	if request.Kind == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Resource must belong to an Extension")
	}
	r := &api.Resource{
		ID:          identity.NewID(),
		Annotations: *request.Annotations,
		Kind:        request.Kind,
		Payload:     request.Payload,
	}

	err := s.store.Update(func(tx store.Tx) error {
		return store.CreateResource(tx, r)
	})

	switch err {
	case store.ErrNoKind:
		return nil, status.Errorf(codes.InvalidArgument, "Kind %v is not registered", r.Kind)
	case store.ErrNameConflict:
		return nil, status.Errorf(
			codes.AlreadyExists,
			"A resource with name %v already exists",
			r.Annotations.Name,
		)
	case nil:
		log.G(ctx).WithFields(logrus.Fields{
			"resource.Name": r.Annotations.Name,
			"method":        "CreateResource",
		}).Debugf("resource created")
		return &api.CreateResourceResponse{Resource: r}, nil
	default:
		return nil, err
	}
}

// GetResource returns a `GetResourceResponse` with a `Resource` with the same
// id as `GetResourceRequest.Resource`
// - Returns `NotFound` if the Resource with the given id is not found.
// - Returns `InvalidArgument` if the `GetResourceRequest.Resource` is empty.
// - Returns an error if getting fails.
func (s *Server) GetResource(ctx context.Context, request *api.GetResourceRequest) (*api.GetResourceResponse, error) {
	if request.ResourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "resource ID must be present")
	}
	var resource *api.Resource
	s.store.View(func(tx store.ReadTx) {
		resource = store.GetResource(tx, request.ResourceID)
	})

	if resource == nil {
		return nil, status.Errorf(codes.NotFound, "resource %s not found", request.ResourceID)
	}

	return &api.GetResourceResponse{Resource: resource}, nil
}

// RemoveResource removes the `Resource` referenced by `RemoveResourceRequest.ResourceID`.
// - Returns `InvalidArgument` if `RemoveResourceRequest.ResourceID` is empty.
// - Returns `NotFound` if the a resource named `RemoveResourceRequest.ResourceID` is not found.
// - Returns an error if the deletion fails.
func (s *Server) RemoveResource(ctx context.Context, request *api.RemoveResourceRequest) (*api.RemoveResourceResponse, error) {
	if request.ResourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "resource ID must be present")
	}
	err := s.store.Update(func(tx store.Tx) error {
		return store.DeleteResource(tx, request.ResourceID)
	})
	switch err {
	case store.ErrNotExist:
		return nil, status.Errorf(codes.NotFound, "resource %s not found", request.ResourceID)
	case nil:
		return &api.RemoveResourceResponse{}, nil
	default:
		return nil, err
	}
}

// ListResources returns a `ListResourcesResponse` with a list of `Resource`s stored in the raft store,
// or all resources matching any name in `ListConfigsRequest.Names`, any
// name prefix in `ListResourcesRequest.NamePrefixes`, any id in
// `ListResourcesRequest.ResourceIDs`, or any id prefix in `ListResourcesRequest.IDPrefixes`.
// - Returns an error if listing fails.
func (s *Server) ListResources(ctx context.Context, request *api.ListResourcesRequest) (*api.ListResourcesResponse, error) {
	var (
		resources     []*api.Resource
		respResources []*api.Resource
		err           error
		byFilters     []store.By
		by            store.By
		labels        map[string]string
	)

	// andKind is set to true if the Extension filter is not the only filter
	// being used. If this is the case, we do not have to compare by strings,
	// which could be slow.
	var andKind bool

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
		if request.Filters.Kind != "" {
			// if we're filtering on Extensions, then set this to true. If Kind is
			// the _only_ kind of filter, we'll set this to false below.
			andKind = true
		}
	}

	switch len(byFilters) {
	case 0:
		// NOTE(dperny): currently, filtering using store.ByKind would apply an
		// Or operation, which means that filtering by kind would return a
		// union. However, for Kind filters, we actually want the
		// _intersection_; that is, _only_ objects of the specified kind. we
		// could dig into the db code to figure out how to write and use an
		// efficient And combinator, but I don't have the time nor expertise to
		// do so at the moment. instead, we'll filter by kind after the fact.
		// however, if there are no other kinds of filters, and we're ONLY
		// listing by Kind, we can set that to be the only filter.
		if andKind {
			by = store.ByKind(request.Filters.Kind)
			andKind = false
		} else {
			by = store.All
		}
	case 1:
		by = byFilters[0]
	default:
		by = store.Or(byFilters...)
	}

	s.store.View(func(tx store.ReadTx) {
		resources, err = store.FindResources(tx, by)
	})
	if err != nil {
		return nil, err
	}

	// filter by label and extension
	for _, resource := range resources {
		if !filterMatchLabels(resource.Annotations.Labels, labels) {
			continue
		}
		if andKind && resource.Kind != request.Filters.Kind {
			continue
		}
		respResources = append(respResources, resource)
	}

	return &api.ListResourcesResponse{Resources: respResources}, nil
}

// UpdateResource updates the resource with the given `UpdateResourceRequest.Resource.Id` using the given `UpdateResourceRequest.Resource` and returns a `UpdateResourceResponse`.
// - Returns `NotFound` if the Resource with the given `UpdateResourceRequest.Resource.Id` is not found.
// - Returns `InvalidArgument` if the UpdateResourceRequest.Resource.Id` is empty.
// - Returns an error if updating fails.
func (s *Server) UpdateResource(ctx context.Context, request *api.UpdateResourceRequest) (*api.UpdateResourceResponse, error) {
	if request.ResourceID == "" || request.ResourceVersion == nil {
		return nil, status.Errorf(codes.InvalidArgument, "must include ID and version")
	}
	var r *api.Resource
	err := s.store.Update(func(tx store.Tx) error {
		r = store.GetResource(tx, request.ResourceID)
		if r == nil {
			return status.Errorf(codes.NotFound, "resource %v not found", request.ResourceID)
		}

		if request.Annotations != nil {
			if r.Annotations.Name != request.Annotations.Name {
				return status.Errorf(codes.InvalidArgument, "cannot change resource name")
			}
			r.Annotations = *request.Annotations
		}
		r.Meta.Version = *request.ResourceVersion
		// only alter the payload if the
		if request.Payload != nil {
			r.Payload = request.Payload
		}

		return store.UpdateResource(tx, r)
	})
	switch err {
	case store.ErrSequenceConflict:
		return nil, status.Errorf(codes.InvalidArgument, "update out of sequence")
	case nil:
		return &api.UpdateResourceResponse{
			Resource: r,
		}, nil
	default:
		return nil, err
	}
}
