// Package image generates container and service names from image references.
//
// Names use the familiar image name with path separators replaced by dashes.
// Container names append a prefix of the container ID that grows on retries.
// Service names append four random bytes encoded as hexadecimal.
package image

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/distribution/reference"
	"github.com/moby/extensions"
	containernamegeneratorv0 "github.com/moby/moby/v2/extpoints/containernamegenerator/v0"
	servicenamegeneratorv0 "github.com/moby/moby/v2/extpoints/servicenamegenerator/v0"
)

// ID is the extension ID and binary name.
const ID = "org.mobyproject.namesgenerator.image.v1"

type generator struct{}

func (generator) GenerateContainerName(_ context.Context, req *containernamegeneratorv0.GenerateContainerNameRequest) (*containernamegeneratorv0.GenerateContainerNameReply, error) {
	if req == nil {
		return nil, errors.New("generate container name request is required")
	}

	const initialIDLength = 4
	idLength := initialIDLength + int(req.Retry)*2
	if len(req.ContainerID) < idLength {
		return nil, fmt.Errorf("container ID must contain at least %d characters", idLength)
	}

	name, err := generate(req.Image, req.ContainerID[:idLength])
	if err != nil {
		return nil, err
	}
	return &containernamegeneratorv0.GenerateContainerNameReply{Name: name}, nil
}

func (generator) GenerateServiceName(_ context.Context, req *servicenamegeneratorv0.GenerateServiceNameRequest) (*servicenamegeneratorv0.GenerateServiceNameReply, error) {
	if req == nil {
		return nil, errors.New("generate service name request is required")
	}

	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return nil, fmt.Errorf("generate service name suffix: %w", err)
	}
	name, err := generate(req.Image, hex.EncodeToString(suffix[:]))
	if err != nil {
		return nil, err
	}
	return &servicenamegeneratorv0.GenerateServiceNameReply{Name: name}, nil
}

func generate(image, suffix string) (string, error) {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return "", fmt.Errorf("parse image reference %q: %w", image, err)
	}
	imageName := strings.ReplaceAll(reference.FamiliarName(ref), "/", "-")
	return imageName + "-" + suffix, nil
}

// Extension provides container and service names derived from their image.
var Extension = extensions.New(extensions.Declaration{
	ID: ID,
	Providers: []extensions.Provider{
		containernamegeneratorv0.Point.Provide(generator{}),
		servicenamegeneratorv0.Point.Provide(generator{}),
	},
})
