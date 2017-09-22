package cgroups

// Hierarchy enableds both unified and split hierarchy for cgroups
type Hierarchy func() ([]Subsystem, error)
