package dockerfile

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/platforms"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/v2/errdefs"
	"github.com/moby/sys/user"
	"golang.org/x/sys/windows"
)

// seTakeOwnershipPrivilege is "SE_TAKE_OWNERSHIP_NAME" in the win32 API.
//
// see https://learn.microsoft.com/en-us/windows/win32/secauthz/privilege-constants
const seTakeOwnershipPrivilege = "SeTakeOwnershipPrivilege"

// Constants for well-known SIDs in the Windows container.
// These are currently undocumented.
// See https://github.com/moby/buildkit/pull/5791#discussion_r1976652227 for more information.
const (
	containerAdministratorSidString = "S-1-5-93-2-1" // ContainerAdministrator
	containerUserSidString          = "S-1-5-93-2-2" // ContainerUser
)

func parseChownFlag(ctx context.Context, builder *Builder, state *dispatchState, chown, ctrRootPath string, identityMapping user.IdentityMapping) (identity, error) {
	if builder.options.Platform == "windows" {
		return getAccountIdentity(ctx, builder, chown, ctrRootPath, state)
	}

	uid, gid := identityMapping.RootPair()
	return identity{UID: uid, GID: gid}, nil
}

func getAccountIdentity(ctx context.Context, builder *Builder, accountName string, ctrRootPath string, state *dispatchState) (identity, error) {
	// If this is potentially a string SID then attempt to convert it to verify
	// this, otherwise continue looking for the account.
	if strings.HasPrefix(accountName, "S-") || strings.HasPrefix(accountName, "s-") {
		sid, err := windows.StringToSid(accountName)

		if err == nil {
			return identity{SID: sid.String()}, nil
		}
	}

	// Attempt to obtain the SID using the name.
	sid, _, accType, err := windows.LookupSID("", accountName)

	// If this is a SID that is built-in and hence the same across all systems then use that.
	if err == nil && (accType == windows.SidTypeAlias || accType == windows.SidTypeWellKnownGroup) {
		return identity{SID: sid.String()}, nil
	}

	// Check if the account name is one unique to containers.
	if strings.EqualFold(accountName, "ContainerAdministrator") {
		return identity{SID: containerAdministratorSidString}, nil
	} else if strings.EqualFold(accountName, "ContainerUser") {
		return identity{SID: containerUserSidString}, nil
	}

	// All other lookups failed, so therefore determine if the account in
	// question exists in the container and if so, obtain its SID.
	return lookupNTAccount(ctx, builder, accountName, state)
}

func lookupNTAccount(ctx context.Context, builder *Builder, accountName string, state *dispatchState) (identity, error) {
	source, _ := filepath.Split(os.Args[0])

	target := "C:\\Docker"
	targetExecutable := target + "\\containerutility.exe"

	optionsPlatform, err := platforms.Parse(builder.options.Platform)
	if err != nil {
		return identity{}, errdefs.InvalidParameter(err)
	}

	runConfig := copyRunConfig(state.runConfig,
		withCmdCommentString("internal run to obtain NT account information.", optionsPlatform.OS))

	runConfig.Cmd = []string{targetExecutable, "getaccountsid", accountName}

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   source,
				Target:   target,
				ReadOnly: true,
			},
		},
	}

	container, err := builder.containerManager.Create(ctx, runConfig, hostConfig)
	if err != nil {
		return identity{}, err
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	if err := builder.containerManager.Run(ctx, container.ID, stdout, stderr); err != nil {
		if err, ok := err.(*statusCodeError); ok {
			return identity{}, &jsonstream.Error{
				Message: stderr.String(),
				Code:    err.StatusCode(),
			}
		}
		return identity{}, err
	}

	accountSid := stdout.String()

	return identity{SID: accountSid}, nil
}
