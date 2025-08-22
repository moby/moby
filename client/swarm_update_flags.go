package client

// UpdateFlags contains flags for SwarmUpdate.
type UpdateFlags struct {
	RotateWorkerToken      bool
	RotateManagerToken     bool
	RotateManagerUnlockKey bool
}
