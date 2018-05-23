package image

// PruneReport contains the response for Engine API:
// POST "/images/prune"
type PruneReport struct {
	ImagesDeleted  []DeleteResponseItem
	SpaceReclaimed uint64
}

// BuildCachePruneReport contains the response for Engine API:
// POST "/build/prune"
type BuildCachePruneReport struct {
	SpaceReclaimed uint64
}
