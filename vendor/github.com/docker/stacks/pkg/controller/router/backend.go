package router

import "github.com/docker/stacks/pkg/types"

// Backend abstracts the Stacks API.
type Backend interface {
	CreateStack(types.StackCreate) (types.StackCreateResponse, error)
	GetStack(id string) (types.Stack, error)
	ListStacks() ([]types.Stack, error)
	UpdateStack(id string, spec types.StackSpec, version uint64) error
	DeleteStack(id string) error
	ParseComposeInput(types.ComposeInput) (*types.StackCreate, error)
}
