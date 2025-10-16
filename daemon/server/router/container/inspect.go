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
		if bridgeNw := ctr.NetworkSettings.Networks["bridge"]; bridgeNw != nil {
			// Old API versions showed the bridge's configuration as top-level
			// fields in "NetworkConfig".
			//
			// This was deprecated in API v1.44, but kept in place until
			// API v1.52, which removes this entirely.
			wrapOpts = append(wrapOpts, compat.WithExtraFields(map[string]any{
				"NetworkSettings": map[string]any{
					"EndpointID":          bridgeNw.EndpointID,
					"Gateway":             bridgeNw.Gateway,
					"GlobalIPv6Address":   bridgeNw.GlobalIPv6Address,
					"GlobalIPv6PrefixLen": bridgeNw.GlobalIPv6PrefixLen,
					"IPAddress":           bridgeNw.IPAddress,
					"IPPrefixLen":         bridgeNw.IPPrefixLen,
					"IPv6Gateway":         bridgeNw.IPv6Gateway,
					"MacAddress":          bridgeNw.MacAddress,
				},
			}))
		}
	}

	return httputils.WriteJSON(w, http.StatusOK, compat.Wrap(ctr, wrapOpts...))
}
