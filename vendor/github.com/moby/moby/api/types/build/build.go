package build

// BuilderVersion sets the version of underlying builder to use
type BuilderVersion string

const (
	// BuilderV1 is the first generation builder in docker daemon
	BuilderV1 BuilderVersion = "1"
	// BuilderBuildKit is builder based on moby/buildkit project
	BuilderBuildKit BuilderVersion = "2"
)

// Result contains the image id of a successful build.
type Result struct {
	ID string
}
