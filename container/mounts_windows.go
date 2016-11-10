package container

// Mount contains information for a mount operation.
type Mount struct {
	Source       string `json:"source"`
	Destination  string `json:"destination"`
	Writable     bool   `json:"writable"`
	MaxBandwidth uint64 `json:"max_bandwidth"`
	MaxIOps      uint64 `json:"max_iops"`
}
