package container

import (
	"context"
	"net/http"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/storage"
	"github.com/moby/moby/v2/daemon/internal/compat"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/moby/moby/v2/daemon/internal/versions"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/daemon/server/httputils"
	"github.com/moby/moby/v2/internal/sliceutil"
)

// getContainersByName inspects container's configuration and serializes it as json.
func (c *containerRouter) getContainersByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	ctr, desiredMACAddress, err := c.backend.ContainerInspect(ctx, vars["name"], backend.ContainerInspectOptions{
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
			// fields in "NetworkSettings".
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

		// Migrate the container's main / default network's MacAddress to
		// the Config.MacAddress field for older API versions (< 1.44).
		//
		// This was deprecated in API v1.44, but kept in place until
		// API v1.52, which removed this entirely.
		if len(desiredMACAddress) != 0 {
			wrapOpts = append(wrapOpts, compat.WithExtraFields(map[string]any{
				"Config": map[string]any{
					"MacAddress": desiredMACAddress,
				},
			}))
		}

		// Restore the GraphDriver field, now omitted when a snapshotter is used.
		// Remove the Storage field that replaced it.
		if ctr.GraphDriver == nil && ctr.Storage != nil && ctr.Storage.RootFS != nil && ctr.Storage.RootFS.Snapshot != nil {
			ctr.GraphDriver = &storage.DriverData{
				Name: ctr.Storage.RootFS.Snapshot.Name,
			}
			ctr.Storage = nil
		}
	}

	if ctr.Config == nil {
		ctr.Config = &container.Config{}
	}
	return httputils.WriteJSON(w, http.StatusOK, compat.Wrap(ctr, wrapOpts...))
}
