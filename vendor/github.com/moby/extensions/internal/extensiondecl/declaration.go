// Package extensiondecl converts and validates declarations reported by
// externally hosted extensions.
package extensiondecl

import (
	"fmt"

	"github.com/moby/extensions"
	servicev0 "github.com/moby/extensions/extpoints/service/v0"
	"github.com/moby/extensions/sdk/sdkapi"
)

// Declaration is the runtime-neutral part of a hosted extension declaration.
type Declaration struct {
	ID               extensions.ExtensionID
	Dependencies     []extensions.Dependency
	Conflicts        []extensions.ExtensionID
	Points           []extensions.PointID
	OfferedPoints    []extensions.PointID
	ProviderServices map[extensions.PointID][]string
}

// Parse converts an SDK declaration and validates the properties shared by all
// externally hosted runtimes.
func Parse(name string, decl *sdkapi.Declaration) (*Declaration, error) {
	if decl == nil || decl.ID == "" {
		return nil, fmt.Errorf("extension %q described no extension", name)
	}
	if decl.ID != name {
		return nil, fmt.Errorf("extension %q declared id %q, which must match its file name", name, decl.ID)
	}

	out := &Declaration{
		ID:               extensions.ExtensionID(decl.ID),
		Dependencies:     Dependencies(decl.Dependencies),
		Conflicts:        Conflicts(decl.Conflicts),
		ProviderServices: Services(decl.ProviderServices),
	}
	for _, point := range decl.Providers {
		out.Points = append(out.Points, extensions.PointID(point.ID))
	}
	if err := ValidateServices(name, out.Points, out.ProviderServices); err != nil {
		return nil, err
	}
	for _, point := range decl.OfferedPoints {
		out.OfferedPoints = append(out.OfferedPoints, extensions.PointID(point))
	}
	if err := ValidateOffers(name, out.Points, out.OfferedPoints, out.ProviderServices); err != nil {
		return nil, err
	}
	return out, nil
}

// Dependencies converts SDK dependencies to host dependencies.
func Dependencies(deps []sdkapi.Dependency) []extensions.Dependency {
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

// Conflicts converts SDK conflict ids to host extension ids.
func Conflicts(ids []string) []extensions.ExtensionID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]extensions.ExtensionID, 0, len(ids))
	for _, id := range ids {
		out = append(out, extensions.ExtensionID(id))
	}
	return out
}

// Services converts the SDK service inventory to services grouped by provider
// point.
func Services(in []sdkapi.ProviderServices) map[extensions.PointID][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[extensions.PointID][]string, len(in))
	for _, services := range in {
		point := extensions.PointID(services.Point)
		if point == "" || len(services.Services) == 0 {
			continue
		}
		out[point] = append(out[point], services.Services...)
	}
	return out
}

// ValidateServices rejects services listed under a point the extension did not
// declare it provides.
func ValidateServices(name string, points []extensions.PointID, services map[extensions.PointID][]string) error {
	declared := make(map[extensions.PointID]bool, len(points))
	for _, point := range points {
		declared[point] = true
	}
	for point := range services {
		if !declared[point] {
			return fmt.Errorf("extension %q serves services for point %q without declaring it", name, point)
		}
	}
	return nil
}

// ValidateOffers rejects malformed, duplicate, unimplemented, and unserved
// offered Points.
func ValidateOffers(name string, points, offered []extensions.PointID, services map[extensions.PointID][]string) error {
	declared := make(map[extensions.PointID]bool, len(points))
	for _, point := range points {
		declared[point] = true
	}
	seen := make(map[extensions.PointID]bool, len(offered))
	for _, point := range offered {
		if err := extensions.ValidatePointID(point); err != nil {
			return fmt.Errorf("extension %q offered an invalid point: %w", name, err)
		}
		if point == servicev0.Point.ID() {
			return fmt.Errorf("extension %q cannot offer publication metadata point %q", name, point)
		}
		if seen[point] {
			return fmt.Errorf("extension %q offered point %q more than once", name, point)
		}
		seen[point] = true
		if !declared[point] {
			return fmt.Errorf("extension %q offered point %q without implementing it", name, point)
		}
		if len(services[point]) == 0 {
			return fmt.Errorf("extension %q offered point %q without reporting a service", name, point)
		}
	}
	return nil
}
