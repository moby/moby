package csi

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/moby/swarmkit/v2/api"
)

// convert.go contains functions for converting swarm objects into CSI requests
// and back again.

// makeTopology converts a swarmkit topology into a CSI topology.
func makeTopologyRequirement(t *api.TopologyRequirement) *csi.TopologyRequirement {
	if t == nil {
		return nil
	}
	return &csi.TopologyRequirement{
		Requisite: makeTopologies(t.Requisite),
		Preferred: makeTopologies(t.Preferred),
	}
}

// makeTopologies converts a slice of swarmkit topologies into a slice of CSI
// topologies.
func makeTopologies(ts []*api.Topology) []*csi.Topology {
	if ts == nil {
		return nil
	}
	csiTops := make([]*csi.Topology, len(ts))
	for i, t := range ts {
		csiTops[i] = makeTopology(t)
	}

	return csiTops
}

// makeTopology converts a swarmkit topology into a CSI topology. These types
// are essentially homologous, with the swarm type being copied verbatim from
// the CSI type (for build reasons).
func makeTopology(t *api.Topology) *csi.Topology {
	if t == nil {
		return nil
	}
	return &csi.Topology{
		Segments: t.Segments,
	}
}

// makeCapcityRange converts the swarmkit CapacityRange object to the
// equivalent CSI object
func makeCapacityRange(cr *api.CapacityRange) *csi.CapacityRange {
	if cr == nil {
		return nil
	}

	return &csi.CapacityRange{
		RequiredBytes: cr.RequiredBytes,
		LimitBytes:    cr.LimitBytes,
	}
}

// unmakeTopologies transforms a CSI-type topology into the equivalent swarm
// type. it is called "unmakeTopologies" because it performs the inverse of
// "makeTopologies".
func unmakeTopologies(topologies []*csi.Topology) []*api.Topology {
	if topologies == nil {
		return nil
	}
	swarmTopologies := make([]*api.Topology, len(topologies))
	for i, t := range topologies {
		swarmTopologies[i] = unmakeTopology(t)
	}
	return swarmTopologies
}

// unmakeTopology transforms a CSI-type topology into the equivalent swarm
// type.
func unmakeTopology(topology *csi.Topology) *api.Topology {
	return &api.Topology{
		Segments: topology.Segments,
	}
}

// makeVolumeInfo converts a csi.Volume object into a swarmkit VolumeInfo
// object.
func makeVolumeInfo(csiVolume *csi.Volume) *api.VolumeInfo {
	return &api.VolumeInfo{
		CapacityBytes:      csiVolume.CapacityBytes,
		VolumeContext:      csiVolume.VolumeContext,
		VolumeID:           csiVolume.VolumeId,
		AccessibleTopology: unmakeTopologies(csiVolume.AccessibleTopology),
	}
}
