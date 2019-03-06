package store

import (
	// TODO(dperny): make better errors
	"github.com/pkg/errors"

	"github.com/containerd/typeurl"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/swarmkit/api"
	gogotypes "github.com/gogo/protobuf/types"

	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/types"
)

// CombinedStack is a struct that holds both a Stack and the post-conversion
// SwarmStack.
type CombinedStack struct {
	Stack      *types.Stack
	SwarmStack *interfaces.SwarmStack
}

func init() {
	typeurl.Register(&CombinedStack{}, "github.com/docker/stacks/CombinedStack")
}

// MarshalStacks takes a Stack objects and marshals it into a protocol buffer
// Any message. Under the hood, this relies on marshaling the objects to JSON.
func MarshalStacks(stack *types.Stack, swarmStack *interfaces.SwarmStack) (*gogotypes.Any, error) {
	// we should first combine the stack and the swarmStack into one object, so
	// they can be marshalled together.
	combinedStack := &CombinedStack{Stack: stack, SwarmStack: swarmStack}
	return typeurl.MarshalAny(combinedStack)

}

// UnmarshalStacks does the MarshalStacks operation in reverse -- takes a proto
// message, and returns the stack and swarmStack contained in it. Note that
// UnmarshalStacks takes a Swarmkit Resource object, instead of an Any proto.
// This is because UnmarshalStacks does the work of updating the fields in the
// Stack (Meta, Version, and ID) that are derrived from the values assigned by
// swarmkit and contained in the Resource
func UnmarshalStacks(resource *api.Resource) (*types.Stack, *interfaces.SwarmStack, error) {
	iface, err := typeurl.UnmarshalAny(resource.Payload)
	if err != nil {
		return nil, nil, err
	}
	// this is a naked cast, which means if for some reason this _isn't_ a
	// CombinedStack object, the program will panic. This is fine, because if
	// such a thing were to occur, it would be panic-worthy.
	combinedStack := iface.(*CombinedStack)

	combinedStack.Stack.ID = resource.ID
	combinedStack.Stack.Version = types.Version{Index: resource.Meta.Version.Index}

	// extract the times from the swarmkit resource message.
	createdAt, err := gogotypes.TimestampFromProto(resource.Meta.CreatedAt)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error converting swarmkit timestamp")
	}
	updatedAt, err := gogotypes.TimestampFromProto(resource.Meta.UpdatedAt)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error converting swarmkit timestamp")
	}

	combinedStack.SwarmStack.ID = resource.ID
	combinedStack.SwarmStack.Meta = swarm.Meta{
		Version: swarm.Version{
			Index: resource.Meta.Version.Index,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	return combinedStack.Stack, combinedStack.SwarmStack, nil
}
