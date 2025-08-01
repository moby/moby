package container

import (
	"context"
	"net/http"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/versions"
	"github.com/moby/moby/v2/daemon/internal/sliceutil"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/daemon/server/httputils"
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

	return httputils.WriteJSON(w, http.StatusOK, ctr)
}
