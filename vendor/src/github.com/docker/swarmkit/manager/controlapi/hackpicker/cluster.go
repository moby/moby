package hackpicker

// AddrSelector is interface which should track cluster for its leader address.
type AddrSelector interface {
	LeaderAddr() (string, error)
}

// RaftCluster is interface which combines useful methods for clustering.
type RaftCluster interface {
	AddrSelector
	IsLeader() bool
}
