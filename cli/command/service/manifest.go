package service

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func resolveServiceImageDigest(dockerCli *command.DockerCli, service *swarm.ServiceSpec, encodedRegistryAuth string) error {
	if !command.IsTrusted() {
		// Contact registry to get manifest
		apiClient := dockerCli.Client()
		manifestInspect, err := apiClient.ImageManifest(context.Background(), service.TaskTemplate.ContainerSpec.Image, encodedRegistryAuth)
		if err != nil {
			return err
		}

		service.TaskTemplate.ContainerSpec.Image = string(manifestInspect.Digest)
		// TODO(nishanttotla): Refactoring to update platform information

		return nil
	}

	ref, err := reference.ParseAnyReference(service.TaskTemplate.ContainerSpec.Image)
	if err != nil {
		return errors.Wrapf(err, "invalid reference %s", service.TaskTemplate.ContainerSpec.Image)
	}

	// If reference does not have digest (is not canonical nor image id)
	if _, ok := ref.(reference.Digested); !ok {
		namedRef, ok := ref.(reference.Named)
		if !ok {
			return errors.New("failed to resolve image digest using content trust: reference is not named")
		}
		namedRef = reference.TagNameOnly(namedRef)
		taggedRef, ok := namedRef.(reference.NamedTagged)
		if !ok {
			return errors.New("failed to resolve image digest using content trust: reference is not tagged")
		}

		resolvedImage, err := trustedResolveDigest(context.Background(), dockerCli, taggedRef)
		if err != nil {
			return errors.Wrap(err, "failed to resolve image digest using content trust")
		}
		resolvedFamiliar := reference.FamiliarString(resolvedImage)
		logrus.Debugf("resolved image tag to %s using content trust", resolvedFamiliar)
		service.TaskTemplate.ContainerSpec.Image = resolvedFamiliar
	}

	return nil
}
