package image

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/trust"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/notary/client"
	"github.com/docker/notary/tuf/data"
	"golang.org/x/net/context"
)

type target struct {
	name   string
	digest digest.Digest
	size   int64
}

// trustedPush handles content trust pushing of an image
func trustedPush(ctx context.Context, cli *command.DockerCli, repoInfo *registry.RepositoryInfo, ref reference.Named, authConfig types.AuthConfig, requestPrivilege types.RequestPrivilegeFunc) error {
	responseBody, err := imagePushPrivileged(ctx, cli, authConfig, ref.String(), requestPrivilege)
	if err != nil {
		return err
	}

	defer responseBody.Close()

	return PushTrustedReference(cli, repoInfo, ref, authConfig, responseBody)
}

// PushTrustedReference pushes a canonical reference to the trust server.
func PushTrustedReference(cli *command.DockerCli, repoInfo *registry.RepositoryInfo, ref reference.Named, authConfig types.AuthConfig, in io.Reader) error {
	// If it is a trusted push we would like to find the target entry which match the
	// tag provided in the function and then do an AddTarget later.
	target := &client.Target{}
	// Count the times of calling for handleTarget,
	// if it is called more that once, that should be considered an error in a trusted push.
	cnt := 0
	handleTarget := func(aux *json.RawMessage) {
		cnt++
		if cnt > 1 {
			// handleTarget should only be called one. This will be treated as an error.
			return
		}

		var pushResult types.PushResult
		err := json.Unmarshal(*aux, &pushResult)
		if err == nil && pushResult.Tag != "" {
			if dgst, err := digest.ParseDigest(pushResult.Digest); err == nil {
				h, err := hex.DecodeString(dgst.Hex())
				if err != nil {
					target = nil
					return
				}
				target.Name = pushResult.Tag
				target.Hashes = data.Hashes{string(dgst.Algorithm()): h}
				target.Length = int64(pushResult.Size)
			}
		}
	}

	var tag string
	switch x := ref.(type) {
	case reference.Canonical:
		return errors.New("cannot push a digest reference")
	case reference.NamedTagged:
		tag = x.Tag()
	default:
		// We want trust signatures to always take an explicit tag,
		// otherwise it will act as an untrusted push.
		if err := jsonmessage.DisplayJSONMessagesToStream(in, cli.Out(), nil); err != nil {
			return err
		}
		fmt.Fprintln(cli.Out(), "No tag specified, skipping trust metadata push")
		return nil
	}

	if err := jsonmessage.DisplayJSONMessagesToStream(in, cli.Out(), handleTarget); err != nil {
		return err
	}

	if cnt > 1 {
		return fmt.Errorf("internal error: only one call to handleTarget expected")
	}

	if target == nil {
		fmt.Fprintln(cli.Out(), "No targets found, please provide a specific tag in order to sign it")
		return nil
	}

	fmt.Fprintln(cli.Out(), "Signing and pushing trust metadata")

	repo, err := trust.GetNotaryRepository(cli, repoInfo, authConfig, "push", "pull")
	if err != nil {
		fmt.Fprintf(cli.Out(), "Error establishing connection to notary repository: %s\n", err)
		return err
	}

	// get the latest repository metadata so we can figure out which roles to sign
	err = repo.Update(false)

	switch err.(type) {
	case client.ErrRepoNotInitialized, client.ErrRepositoryNotExist:
		keys := repo.CryptoService.ListKeys(data.CanonicalRootRole)
		var rootKeyID string
		// always select the first root key
		if len(keys) > 0 {
			sort.Strings(keys)
			rootKeyID = keys[0]
		} else {
			rootPublicKey, err := repo.CryptoService.Create(data.CanonicalRootRole, "", data.ECDSAKey)
			if err != nil {
				return err
			}
			rootKeyID = rootPublicKey.ID()
		}

		// Initialize the notary repository with a remotely managed snapshot key
		if err := repo.Initialize([]string{rootKeyID}, data.CanonicalSnapshotRole); err != nil {
			return trust.NotaryError(repoInfo.FullName(), err)
		}
		fmt.Fprintf(cli.Out(), "Finished initializing %q\n", repoInfo.FullName())
		err = repo.AddTarget(target, data.CanonicalTargetsRole)
	case nil:
		// already initialized and we have successfully downloaded the latest metadata
		err = addTargetToAllSignableRoles(repo, target)
	default:
		return trust.NotaryError(repoInfo.FullName(), err)
	}

	if err == nil {
		err = repo.Publish()
	}

	if err != nil {
		fmt.Fprintf(cli.Out(), "Failed to sign %q:%s - %s\n", repoInfo.FullName(), tag, err.Error())
		return trust.NotaryError(repoInfo.FullName(), err)
	}

	fmt.Fprintf(cli.Out(), "Successfully signed %q:%s\n", repoInfo.FullName(), tag)
	return nil
}

// Attempt to add the image target to all the top level delegation roles we can
// (based on whether we have the signing key and whether the role's path allows
// us to).
// If there are no delegation roles, we add to the targets role.
func addTargetToAllSignableRoles(repo *client.NotaryRepository, target *client.Target) error {
	var signableRoles []string

	// translate the full key names, which includes the GUN, into just the key IDs
	allCanonicalKeyIDs := make(map[string]struct{})
	for fullKeyID := range repo.CryptoService.ListAllKeys() {
		allCanonicalKeyIDs[path.Base(fullKeyID)] = struct{}{}
	}

	allDelegationRoles, err := repo.GetDelegationRoles()
	if err != nil {
		return err
	}

	// if there are no delegation roles, then just try to sign it into the targets role
	if len(allDelegationRoles) == 0 {
		return repo.AddTarget(target, data.CanonicalTargetsRole)
	}

	// there are delegation roles, find every delegation role we have a key for, and
	// attempt to sign into into all those roles.
	for _, delegationRole := range allDelegationRoles {
		// We do not support signing any delegation role that isn't a direct child of the targets role.
		// Also don't bother checking the keys if we can't add the target
		// to this role due to path restrictions
		if path.Dir(delegationRole.Name) != data.CanonicalTargetsRole || !delegationRole.CheckPaths(target.Name) {
			continue
		}

		for _, canonicalKeyID := range delegationRole.KeyIDs {
			if _, ok := allCanonicalKeyIDs[canonicalKeyID]; ok {
				signableRoles = append(signableRoles, delegationRole.Name)
				break
			}
		}
	}

	if len(signableRoles) == 0 {
		return fmt.Errorf("no valid signing keys for delegation roles")
	}

	return repo.AddTarget(target, signableRoles...)
}

// imagePushPrivileged push the image
func imagePushPrivileged(ctx context.Context, cli *command.DockerCli, authConfig types.AuthConfig, ref string, requestPrivilege types.RequestPrivilegeFunc) (io.ReadCloser, error) {
	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return nil, err
	}
	options := types.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}

	return cli.Client().ImagePush(ctx, ref, options)
}

// trustedPull handles content trust pulling of an image
func trustedPull(ctx context.Context, cli *command.DockerCli, repoInfo *registry.RepositoryInfo, ref reference.Named, authConfig types.AuthConfig, requestPrivilege types.RequestPrivilegeFunc) error {
	var refs []target

	notaryRepo, err := trust.GetNotaryRepository(cli, repoInfo, authConfig, "pull")
	if err != nil {
		fmt.Fprintf(cli.Out(), "Error establishing connection to trust repository: %s\n", err)
		return err
	}

	if tagged, isTagged := ref.(reference.NamedTagged); !isTagged {
		// List all targets
		targets, err := notaryRepo.ListTargets(trust.ReleasesRole, data.CanonicalTargetsRole)
		if err != nil {
			return trust.NotaryError(repoInfo.FullName(), err)
		}
		for _, tgt := range targets {
			t, err := convertTarget(tgt.Target)
			if err != nil {
				fmt.Fprintf(cli.Out(), "Skipping target for %q\n", repoInfo.Name())
				continue
			}
			// Only list tags in the top level targets role or the releases delegation role - ignore
			// all other delegation roles
			if tgt.Role != trust.ReleasesRole && tgt.Role != data.CanonicalTargetsRole {
				continue
			}
			refs = append(refs, t)
		}
		if len(refs) == 0 {
			return trust.NotaryError(repoInfo.FullName(), fmt.Errorf("No trusted tags for %s", repoInfo.FullName()))
		}
	} else {
		t, err := notaryRepo.GetTargetByName(tagged.Tag(), trust.ReleasesRole, data.CanonicalTargetsRole)
		if err != nil {
			return trust.NotaryError(repoInfo.FullName(), err)
		}
		// Only get the tag if it's in the top level targets role or the releases delegation role
		// ignore it if it's in any other delegation roles
		if t.Role != trust.ReleasesRole && t.Role != data.CanonicalTargetsRole {
			return trust.NotaryError(repoInfo.FullName(), fmt.Errorf("No trust data for %s", tagged.Tag()))
		}

		logrus.Debugf("retrieving target for %s role\n", t.Role)
		r, err := convertTarget(t.Target)
		if err != nil {
			return err

		}
		refs = append(refs, r)
	}

	for i, r := range refs {
		displayTag := r.name
		if displayTag != "" {
			displayTag = ":" + displayTag
		}
		fmt.Fprintf(cli.Out(), "Pull (%d of %d): %s%s@%s\n", i+1, len(refs), repoInfo.Name(), displayTag, r.digest)

		ref, err := reference.WithDigest(reference.TrimNamed(repoInfo), r.digest)
		if err != nil {
			return err
		}
		if err := imagePullPrivileged(ctx, cli, authConfig, ref.String(), requestPrivilege, false); err != nil {
			return err
		}

		tagged, err := reference.WithTag(repoInfo, r.name)
		if err != nil {
			return err
		}
		trustedRef, err := reference.WithDigest(reference.TrimNamed(repoInfo), r.digest)
		if err != nil {
			return err
		}
		if err := TagTrusted(ctx, cli, trustedRef, tagged); err != nil {
			return err
		}
	}
	return nil
}

// imagePullPrivileged pulls the image and displays it to the output
func imagePullPrivileged(ctx context.Context, cli *command.DockerCli, authConfig types.AuthConfig, ref string, requestPrivilege types.RequestPrivilegeFunc, all bool) error {

	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}
	options := types.ImagePullOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
		All:           all,
	}

	responseBody, err := cli.Client().ImagePull(ctx, ref, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesToStream(responseBody, cli.Out(), nil)
}

// TrustedReference returns the canonical trusted reference for an image reference
func TrustedReference(ctx context.Context, cli *command.DockerCli, ref reference.NamedTagged, rs registry.Service) (reference.Canonical, error) {
	var (
		repoInfo *registry.RepositoryInfo
		err      error
	)
	if rs != nil {
		repoInfo, err = rs.ResolveRepository(ref)
	} else {
		repoInfo, err = registry.ParseRepositoryInfo(ref)
	}
	if err != nil {
		return nil, err
	}

	// Resolve the Auth config relevant for this server
	authConfig := command.ResolveAuthConfig(ctx, cli, repoInfo.Index)

	notaryRepo, err := trust.GetNotaryRepository(cli, repoInfo, authConfig, "pull")
	if err != nil {
		fmt.Fprintf(cli.Out(), "Error establishing connection to trust repository: %s\n", err)
		return nil, err
	}

	t, err := notaryRepo.GetTargetByName(ref.Tag(), trust.ReleasesRole, data.CanonicalTargetsRole)
	if err != nil {
		return nil, trust.NotaryError(repoInfo.FullName(), err)
	}
	// Only list tags in the top level targets role or the releases delegation role - ignore
	// all other delegation roles
	if t.Role != trust.ReleasesRole && t.Role != data.CanonicalTargetsRole {
		return nil, trust.NotaryError(repoInfo.FullName(), fmt.Errorf("No trust data for %s", ref.Tag()))
	}
	r, err := convertTarget(t.Target)
	if err != nil {
		return nil, err

	}

	return reference.WithDigest(reference.TrimNamed(ref), r.digest)
}

func convertTarget(t client.Target) (target, error) {
	h, ok := t.Hashes["sha256"]
	if !ok {
		return target{}, errors.New("no valid hash, expecting sha256")
	}
	return target{
		name:   t.Name,
		digest: digest.NewDigestFromHex("sha256", hex.EncodeToString(h)),
		size:   t.Length,
	}, nil
}

// TagTrusted tags a trusted ref
func TagTrusted(ctx context.Context, cli *command.DockerCli, trustedRef reference.Canonical, ref reference.NamedTagged) error {
	fmt.Fprintf(cli.Out(), "Tagging %s as %s\n", trustedRef.String(), ref.String())

	return cli.Client().ImageTag(ctx, trustedRef.String(), ref.String())
}
