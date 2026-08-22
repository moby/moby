package launcher

import (
	"fmt"

	"github.com/moby/extensions"
	"github.com/moby/extensions/sdk/sdkapi"
)

// validateDeclaredServices rejects services listed under a point the extension
// did not declare it provides.
func validateDeclaredServices(name string, launched *Launched) error {
	declared := make(map[extensions.PointID]bool, len(launched.Points))
	for _, p := range launched.Points {
		declared[p.ID] = true
	}
	for point := range launched.ProviderServices {
		if !declared[point] {
			return fmt.Errorf("extension %q serves services for point %q without declaring it", name, point)
		}
	}
	return nil
}

// declaredServices converts the service inventory reported by the SDK:
// service names grouped by the provider point whose ServerPoint registered them.
func declaredServices(in []sdkapi.ProviderServices) map[extensions.PointID][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[extensions.PointID][]string, len(in))
	for _, ps := range in {
		point := extensions.PointID(ps.Point)
		if point == "" || len(ps.Services) == 0 {
			continue
		}
		out[point] = append(out[point], ps.Services...)
	}
	return out
}

// declaredDependencies converts declared dependencies to extension dependencies.
func declaredDependencies(deps []sdkapi.Dependency) []extensions.Dependency {
	if len(deps) == 0 {
		return nil
	}
	out := make([]extensions.Dependency, 0, len(deps))
	for _, dep := range deps {
		out = append(out, extensions.Dependency{
			Point:     extensions.PointID(dep.Point),
			Extension: extensions.ExtensionID(dep.Extension),
			Optional:  dep.Optional,
		})
	}
	return out
}

// declaredConflicts converts declared conflict ids to extension ids.
func declaredConflicts(ids []string) []extensions.ExtensionID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]extensions.ExtensionID, 0, len(ids))
	for _, id := range ids {
		out = append(out, extensions.ExtensionID(id))
	}
	return out
}
