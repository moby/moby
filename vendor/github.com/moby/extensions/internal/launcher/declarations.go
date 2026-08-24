package launcher

import (
	"github.com/moby/extensions"
	"github.com/moby/extensions/internal/extensiondecl"
	"github.com/moby/extensions/sdk/sdkapi"
)

// validateDeclaredServices rejects services listed under a point the extension
// did not declare it provides.
func validateDeclaredServices(name string, launched *Launched) error {
	points := make([]extensions.PointID, 0, len(launched.Points))
	for _, point := range launched.Points {
		points = append(points, point.ID)
	}
	return extensiondecl.ValidateServices(name, points, launched.ProviderServices)
}

// declaredServices converts the service inventory reported by the SDK:
// service names grouped by the provider point whose ServerPoint registered them.
func declaredServices(in []sdkapi.ProviderServices) map[extensions.PointID][]string {
	return extensiondecl.Services(in)
}

// declaredDependencies converts declared dependencies to extension dependencies.
func declaredDependencies(deps []sdkapi.Dependency) []extensions.Dependency {
	return extensiondecl.Dependencies(deps)
}

// declaredConflicts converts declared conflict ids to extension ids.
func declaredConflicts(ids []string) []extensions.ExtensionID {
	return extensiondecl.Conflicts(ids)
}
