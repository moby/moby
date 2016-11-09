package store

import "github.com/docker/swarmkit/api"

// By is an interface type passed to Find methods. Implementations must be
// defined in this package.
type By interface {
	// isBy allows this interface to only be satisfied by certain internal
	// types.
	isBy()
}

type byAll struct{}

func (a byAll) isBy() {
}

// All is an argument that can be passed to find to list all items in the
// set.
var All byAll

type byNamePrefix string

func (b byNamePrefix) isBy() {
}

// ByNamePrefix creates an object to pass to Find to select by query.
func ByNamePrefix(namePrefix string) By {
	return byNamePrefix(namePrefix)
}

type byIDPrefix string

func (b byIDPrefix) isBy() {
}

// ByIDPrefix creates an object to pass to Find to select by query.
func ByIDPrefix(idPrefix string) By {
	return byIDPrefix(idPrefix)
}

type byName string

func (b byName) isBy() {
}

// ByName creates an object to pass to Find to select by name.
func ByName(name string) By {
	return byName(name)
}

type byCN string

func (b byCN) isBy() {
}

// ByCN creates an object to pass to Find to select by CN.
func ByCN(name string) By {
	return byCN(name)
}

type byService string

func (b byService) isBy() {
}

// ByServiceID creates an object to pass to Find to select by service.
func ByServiceID(serviceID string) By {
	return byService(serviceID)
}

type byNode string

func (b byNode) isBy() {
}

// ByNodeID creates an object to pass to Find to select by node.
func ByNodeID(nodeID string) By {
	return byNode(nodeID)
}

type bySlot struct {
	serviceID string
	slot      uint64
}

func (b bySlot) isBy() {
}

// BySlot creates an object to pass to Find to select by slot.
func BySlot(serviceID string, slot uint64) By {
	return bySlot{serviceID: serviceID, slot: slot}
}

type byDesiredState api.TaskState

func (b byDesiredState) isBy() {
}

// ByDesiredState creates an object to pass to Find to select by desired state.
func ByDesiredState(state api.TaskState) By {
	return byDesiredState(state)
}

type byTaskState api.TaskState

func (b byTaskState) isBy() {
}

// ByTaskState creates an object to pass to Find to select by task state.
func ByTaskState(state api.TaskState) By {
	return byTaskState(state)
}

type byRole api.NodeRole

func (b byRole) isBy() {
}

// ByRole creates an object to pass to Find to select by role.
func ByRole(role api.NodeRole) By {
	return byRole(role)
}

type byMembership api.NodeSpec_Membership

func (b byMembership) isBy() {
}

// ByMembership creates an object to pass to Find to select by Membership.
func ByMembership(membership api.NodeSpec_Membership) By {
	return byMembership(membership)
}

type byReferencedNetworkID string

func (b byReferencedNetworkID) isBy() {
}

// ByReferencedNetworkID creates an object to pass to Find to search for a
// service or task that references a network with the given ID.
func ByReferencedNetworkID(networkID string) By {
	return byReferencedNetworkID(networkID)
}

type byReferencedSecretID string

func (b byReferencedSecretID) isBy() {
}

// ByReferencedSecretID creates an object to pass to Find to search for a
// service or task that references a secret with the given ID.
func ByReferencedSecretID(secretID string) By {
	return byReferencedSecretID(secretID)
}
