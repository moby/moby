package ebpf

// PossibleCPU returns the max number of CPUs a system may possibly have
// Logical CPU numbers must be of the form 0-n
func PossibleCPU() (int, error) {
	return possibleCPU()
}

// MustPossibleCPU is a helper that wraps a call to PossibleCPU and panics if
// the error is non-nil.
func MustPossibleCPU() int {
	cpus, err := PossibleCPU()
	if err != nil {
		panic(err)
	}
	return cpus
}
