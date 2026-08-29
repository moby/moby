// Package servicev0 defines metadata through which extensions offer ordinary
// Points for Host-controlled publication.
package servicev0

import (
	"fmt"

	"github.com/moby/extensions"
)

// Point is the metadata Point used to offer ordinary Points for publication.
var Point = extensions.DefinePoint[Provider]("org.mobyproject.extension.service.v0")

// Provider reports ordinary Points an extension offers for publication.
type Provider interface {
	OfferedPoints() []extensions.PointID
}

type offeredPoint interface {
	ID() extensions.PointID
}

// Offer declares ordinary Points eligible for Host-controlled publication.
func Offer(points ...offeredPoint) extensions.Provider {
	if len(points) == 0 {
		panic("servicev0: no offered points")
	}
	offered := make([]extensions.PointID, 0, len(points))
	seen := make(map[extensions.PointID]struct{}, len(points))
	for _, point := range points {
		id := point.ID()
		if id == Point.ID() {
			panic(fmt.Sprintf("servicev0: point %q cannot offer itself", id))
		}
		if _, ok := seen[id]; ok {
			panic(fmt.Sprintf("servicev0: duplicate point %q", id))
		}
		seen[id] = struct{}{}
		offered = append(offered, id)
	}
	return Point.Provide(provider{points: offered})
}

type provider struct {
	points []extensions.PointID
}

func (p provider) OfferedPoints() []extensions.PointID {
	return append([]extensions.PointID(nil), p.points...)
}
