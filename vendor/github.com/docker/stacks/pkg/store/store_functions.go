package store

// store_functions.go implements the StackStore methods as functions instead.
// Having a store object has advantages for testing and stand-alone stacks
// development, but having these functions available makes integration with the
// docker engine simpler.

import (
	"context"

	swarmapi "github.com/docker/swarmkit/api"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/types"
)

const (
	// StackResourceKind defines the Kind of swarmkit Resources belonging to
	// stacks.
	StackResourceKind = "github.com/docker/stacks/Stack"
	// StackResourcesDescription is the description string of the stack
	// extension kind.
	StackResourcesDescription = "Docker server-side stacks"
)

// InitExtension initializes the stack resource extension object
func InitExtension(ctx context.Context, rc ResourcesClient) error {
	// try creating the extension
	req := &swarmapi.CreateExtensionRequest{
		Annotations: &swarmapi.Annotations{
			Name: StackResourceKind,
		},
		Description: StackResourcesDescription,
	}
	// we don't actually care about the response -- the only important thing
	// is the error.
	_, err := rc.CreateExtension(ctx, req)
	// we're looking to see if the error we got back is codes.AlreadyExists.
	// That would mean the extension is already created, and we have nothing to
	// do.

	// if this isn't a grpc status error, then return the error. if err is nil,
	// then "ok" will be true, but the resulting status will be one where the
	// code is codes.OK, which won't match the below check
	if s, ok := status.FromError(err); ok {
		// if this codes.AlreadyExists, then no actual error that we care about
		// has occurred.
		if s.Code() == codes.AlreadyExists {
			return nil
		}
		// if the error is nil, this will obviously be nil, even though
		// status.FromError returns ok
		return err
	}

	return err
}

// AddStack adds a stack
func AddStack(ctx context.Context, rc ResourcesClient, st types.Stack, sst interfaces.SwarmStack) (string, error) {
	// first, marshal the stacks to a proto message
	any, err := MarshalStacks(&st, &sst)
	if err != nil {
		return "", err
	}

	// reuse the Annotations from the SwarmStack. However, since they're
	// actually different types, convert them
	annotations := &swarmapi.Annotations{
		Name:   sst.Spec.Annotations.Name,
		Labels: sst.Spec.Annotations.Labels,
	}

	// create a resource creation request
	req := &swarmapi.CreateResourceRequest{
		Annotations: annotations,
		Kind:        StackResourceKind,
		Payload:     any,
	}

	// now create the resource object
	resp, err := rc.CreateResource(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Resource.ID, nil
}

// UpdateStack updates a stack's specs.
func UpdateStack(ctx context.Context, rc ResourcesClient, id string, st types.StackSpec, sst interfaces.SwarmStackSpec, version uint64) error {
	// get the swarmkit resource
	resp, err := rc.GetResource(ctx, &swarmapi.GetResourceRequest{
		ResourceID: id,
	})
	if err != nil {
		return err
	}

	resource := resp.Resource
	// unmarshal the contents
	stack, swarmStack, err := UnmarshalStacks(resource)
	if err != nil {
		return err
	}

	// update the specs
	stack.Spec = st
	swarmStack.Spec = sst

	// marshal it all back
	any, err := MarshalStacks(stack, swarmStack)
	if err != nil {
		return err
	}

	// and then issue an update.
	_, err = rc.UpdateResource(context.TODO(),
		&swarmapi.UpdateResourceRequest{
			ResourceID:      id,
			ResourceVersion: &swarmapi.Version{Index: version},
			Annotations: &swarmapi.Annotations{
				// Swarmkit will return an error if any changes to the
				// name occur.
				Name:   sst.Annotations.Name,
				Labels: sst.Annotations.Labels,
			},
			Payload: any,
		},
	)
	return err
}

// DeleteStack deletes a stack
func DeleteStack(ctx context.Context, rc ResourcesClient, id string) error {
	// this one is easy, no type conversion needed
	_, err := rc.RemoveResource(
		ctx, &swarmapi.RemoveResourceRequest{ResourceID: id},
	)
	return err
}

// GetStack returns a stack
func GetStack(ctx context.Context, rc ResourcesClient, id string) (types.Stack, error) {
	resp, err := rc.GetResource(
		ctx, &swarmapi.GetResourceRequest{ResourceID: id},
	)
	if err != nil {
		return types.Stack{}, err
	}
	resource := resp.Resource

	// now, we have to get the stack out of the resource object
	stack, _, err := UnmarshalStacks(resource)
	if err != nil {
		return types.Stack{}, err
	}
	if stack == nil {
		return types.Stack{}, errors.New("got back an empty stack")
	}

	// and then return the stack
	return *stack, nil
}

// GetSwarmStack returns a swarm stack
func GetSwarmStack(ctx context.Context, rc ResourcesClient, id string) (interfaces.SwarmStack, error) {
	resp, err := rc.GetResource(
		ctx, &swarmapi.GetResourceRequest{ResourceID: id},
	)
	if err != nil {
		return interfaces.SwarmStack{}, err
	}
	resource := resp.Resource
	_, swarmStack, err := UnmarshalStacks(resource)
	if err != nil {
		return interfaces.SwarmStack{}, err
	}
	if swarmStack == nil {
		return interfaces.SwarmStack{}, errors.New("got back an empty stack")
	}
	return *swarmStack, nil
}

// ListStacks returns all stacks
func ListStacks(ctx context.Context, rc ResourcesClient) ([]types.Stack, error) {
	resp, err := rc.ListResources(ctx,
		&swarmapi.ListResourcesRequest{
			Filters: &swarmapi.ListResourcesRequest_Filters{
				// list only stacks
				Kind: StackResourceKind,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	// unmarshal and pack up all of the stack objects
	stacks := make([]types.Stack, 0, len(resp.Resources))
	for _, resource := range resp.Resources {
		stack, _, err := UnmarshalStacks(resource)
		if err != nil {
			return nil, err
		}
		stacks = append(stacks, *stack)
	}
	return stacks, nil
}

// ListSwarmStacks returns all swarm stacks
func ListSwarmStacks(ctx context.Context, rc ResourcesClient) ([]interfaces.SwarmStack, error) {
	resp, err := rc.ListResources(ctx,
		&swarmapi.ListResourcesRequest{
			Filters: &swarmapi.ListResourcesRequest_Filters{
				// list only stacks
				Kind: StackResourceKind,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	stacks := make([]interfaces.SwarmStack, 0, len(resp.Resources))
	for _, resource := range resp.Resources {
		_, stack, err := UnmarshalStacks(resource)
		if err != nil {
			return nil, err
		}
		stacks = append(stacks, *stack)
	}
	return stacks, nil
}
