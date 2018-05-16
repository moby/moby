package network

// PruneReport contains the response for Engine API:
// POST "/networks/prune"
type PruneReport struct {
	NetworksDeleted []string
}
