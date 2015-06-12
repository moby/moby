package sandbox

// GenerateKey generates a sandbox key based on the passed
// container id.
func GenerateKey(containerID string) string {
	maxLen := 12
	if len(containerID) < maxLen {
		maxLen = len(containerID)
	}

	return containerID[:maxLen]
}

// NewSandbox provides a new sandbox instance created in an os specific way
// provided a key which uniquely identifies the sandbox
func NewSandbox(key string, osCreate bool) (Sandbox, error) {
	return nil, nil
}

// GC triggers garbage collection of namespace path right away
// and waits for it.
func GC() {
}
