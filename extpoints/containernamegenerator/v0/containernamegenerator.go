//go:generate go tool mobyextgen

// Package containernamegeneratorv0 defines the experimental point for
// generating default container names.
package containernamegeneratorv0

import (
	"context"
	"errors"
	"fmt"

	"github.com/moby/extensions"
	"github.com/moby/moby/v2/extpoints/internal/namesgenerator"
)

// ContainerNameGenerator generates a default container name.
type ContainerNameGenerator interface {
	GenerateContainerName(ctx context.Context, req *GenerateContainerNameRequest) (*GenerateContainerNameReply, error)
}

// GenerateContainerNameRequest describes a container-name generation attempt.
type GenerateContainerNameRequest struct {
	// Retry is the number of prior generated names that collided.
	Retry int64 `pb:"1"`
	// ContainerID is the ID assigned to the container.
	ContainerID string `pb:"2"`
	// Image is the image reference used to create the container.
	Image string `pb:"3"`
}

// GenerateContainerNameReply contains a generated container name.
type GenerateContainerNameReply struct {
	// Name must contain 2 to 63 ASCII letters, digits, hyphens, or underscores.
	// It must start and end with a letter or digit.
	Name string `pb:"1"`
}

// Point is the single-provider container name-generator point.
//
//mobyextgen:service=ContainerNameGenerator
var Point = extensions.DefineSinglePoint[ContainerNameGenerator]("org.mobyproject.extension.containernamegenerator.v0")

// GenerateContainerName calls the effective container name provider.
func GenerateContainerName(ctx context.Context, resolver extensions.Resolver, req *GenerateContainerNameRequest) (*GenerateContainerNameReply, error) {
	name, err := namesgenerator.Generate(ctx, resolver, Point.ID(), func(ctx context.Context, impl any) (string, error) {
		generator, ok := impl.(ContainerNameGenerator)
		if !ok {
			return "", fmt.Errorf("provider has type %T, not ContainerNameGenerator", impl)
		}
		reply, err := generator.GenerateContainerName(ctx, req)
		if err != nil {
			return "", err
		}
		if reply == nil {
			return "", errors.New("provider returned a nil reply")
		}
		return reply.Name, nil
	})
	if name == "" {
		return nil, err
	}
	return &GenerateContainerNameReply{Name: name}, err
}
