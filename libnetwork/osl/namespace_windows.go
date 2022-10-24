package osl

// GenerateKey generates a sandbox key based on the passed
// container id.
func GenerateKey(containerID string) string {
	return containerID
}

// NewSandbox provides a new sandbox instance created in an os specific way
// provided a key which uniquely identifies the sandbox
func NewSandbox(key string, osCreate, isRestore bool) (Sandbox, error) {
	return nil, nil
}

func GetSandboxForExternalKey(path string, key string) (Sandbox, error) {
	return nil, nil
}

// GC triggers garbage collection of namespace path right away
// and waits for it.
func GC() {
}

// SetBasePath sets the base url prefix for the ns path
func SetBasePath(path string) {
}
