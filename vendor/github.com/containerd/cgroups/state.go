package cgroups

// State is a type that represents the state of the current cgroup
type State string

const (
	Unknown  State = ""
	Thawed   State = "thawed"
	Frozen   State = "frozen"
	Freezing State = "freezing"
	Deleted  State = "deleted"
)
