// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

package container // import "github.com/docker/docker/api/server/router/container"

import (
	"context"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/internal/sliceutil"
	"github.com/docker/docker/pkg/stringid"
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
