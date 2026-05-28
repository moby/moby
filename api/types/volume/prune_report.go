package volume

// PruneReport contains the response for Engine API:
// POST "/volumes/prune"
type PruneReport struct {
	VolumesDeleted []string
	SpaceReclaimed uint64
}
