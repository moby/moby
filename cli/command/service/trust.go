package service

import (
	"encoding/hex"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	distreference "github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/trust"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/notary/tuf/data"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func resolveServiceImageDigest(dockerCli *command.DockerCli, service *swarm.ServiceSpec) error {
	if !command.IsTrusted() {
		// Digests are resolved by the daemon when not using content
		// trust.
		return nil
	}

	image := service.TaskTemplate.ContainerSpec.Image

	// We only attempt to resolve the digest if the reference
	// could be parsed as a digest reference. Specifying an image ID
	// is valid but not resolvable. There is no warning message for
	// an image ID because it's valid to use one.
	if _, err := digest.ParseDigest(image); err == nil {
		return nil
	}

	ref, err := reference.ParseNamed(image)
	if err != nil {
		return fmt.Errorf("Could not parse image reference %s", service.TaskTemplate.ContainerSpec.Image)
	}
	if _, ok := ref.(reference.Canonical); !ok {
		ref = reference.WithDefaultTag(ref)

		taggedRef, ok := ref.(reference.NamedTagged)
		if !ok {
			// This should never happen because a reference either
			// has a digest, or WithDefaultTag would give it a tag.
			return errors.New("Failed to resolve image digest using content trust: reference is missing a tag")
		}

		resolvedImage, err := trustedResolveDigest(context.Background(), dockerCli, taggedRef)
		if err != nil {
			return fmt.Errorf("Failed to resolve image digest using content trust: %v", err)
		}
		logrus.Debugf("resolved image tag to %s using content trust", resolvedImage.String())
		service.TaskTemplate.ContainerSpec.Image = resolvedImage.String()
	}
	return nil
}

func trustedResolveDigest(ctx context.Context, cli *command.DockerCli, ref reference.NamedTagged) (distreference.Canonical, error) {
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, err
	}

	authConfig := command.ResolveAuthConfig(ctx, cli, repoInfo.Index)

	notaryRepo, err := trust.GetNotaryRepository(cli, repoInfo, authConfig, "pull")
	if err != nil {
		return nil, errors.Wrap(err, "error establishing connection to trust repository")
	}

	t, err := notaryRepo.GetTargetByName(ref.Tag(), trust.ReleasesRole, data.CanonicalTargetsRole)
	if err != nil {
		return nil, trust.NotaryError(repoInfo.FullName(), err)
	}
	// Only get the tag if it's in the top level targets role or the releases delegation role
	// ignore it if it's in any other delegation roles
	if t.Role != trust.ReleasesRole && t.Role != data.CanonicalTargetsRole {
		return nil, trust.NotaryError(repoInfo.FullName(), fmt.Errorf("No trust data for %s", ref.String()))
	}

	logrus.Debugf("retrieving target for %s role\n", t.Role)
	h, ok := t.Hashes["sha256"]
	if !ok {
		return nil, errors.New("no valid hash, expecting sha256")
	}

	dgst := digest.NewDigestFromHex("sha256", hex.EncodeToString(h))

	// Using distribution reference package to make sure that adding a
	// digest does not erase the tag. When the two reference packages
	// are unified, this will no longer be an issue.
	return distreference.WithDigest(ref, dgst)
}
