package container

import (
	"context"
	"net/http"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/versions"
	"github.com/moby/moby/v2/daemon/internal/compat"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/daemon/server/httputils"
	"github.com/moby/moby/v2/internal/sliceutil"
)

// getContainersByName inspects container's configuration and serializes it as json.
func (c *containerRouter) getContainersByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	ctr, err := c.backend.ContainerInspect(ctx, vars["name"], backend.ContainerInspectOptions{
		Size: httputils.BoolValue(r, "size"),
	})
	if err != nil {
		return err
	}

	version := httputils.VersionFromContext(ctx)
	if versions.LessThan(version, "1.45") {
		shortCID := stringid.TruncateID(ctr.ID)
		for nwName, ep := range ctr.NetworkSettings.Networks {
			if container.NetworkMode(nwName).IsUserDefined() {
				ep.Aliases = sliceutil.Dedup(append(ep.Aliases, shortCID, ctr.Config.Hostname))
			}
		}
	}
	if versions.LessThan(version, "1.48") {
		ctr.ImageManifestDescriptor = nil
	}

	var wrapOpts []compat.Option
	if versions.LessThan(version, "1.52") {
		var macAddress string
		if bridgeNw := ctr.NetworkSettings.Networks["bridge"]; bridgeNw != nil {
			macAddress = bridgeNw.MacAddress
			// Copy all fields to the top-level, except for MacAddress, for
			// which a custom network takes priority (if used).
			wrapOpts = append(wrapOpts, compat.WithExtraFields(map[string]any{
				"EndpointID":          bridgeNw.EndpointID,
				"Gateway":             bridgeNw.Gateway,
				"GlobalIPv6Address":   bridgeNw.GlobalIPv6Address,
				"GlobalIPv6PrefixLen": bridgeNw.GlobalIPv6PrefixLen,
				"IPAddress":           bridgeNw.IPAddress,
				"IPPrefixLen":         bridgeNw.IPPrefixLen,
				"IPv6Gateway":         bridgeNw.IPv6Gateway,
			}))
		}
		if ctr.HostConfig != nil {
			// Migrate the container's default network's MacAddress to the top-level
			// Config.MacAddress field for older API versions (< 1.44). We set it here
			// unconditionally, to keep backward compatibility with clients that use
			// unversioned API endpoints.
			if nwm := ctr.HostConfig.NetworkMode; nwm.IsBridge() || nwm.IsUserDefined() {
				if v := ctr.NetworkSettings.Networks[nwm.NetworkName()]; v != nil && v.MacAddress != "" {
					macAddress = v.MacAddress
				}
			}
		}
		if macAddress != "" {
			wrapOpts = append(wrapOpts, compat.WithExtraFields(map[string]any{
				"Config": map[string]any{
					"MacAddress": macAddress,
				},
			}))
		}
	}

	return httputils.WriteJSON(w, http.StatusOK, compat.Wrap(ctr, wrapOpts...))
}
