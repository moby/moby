package build

// Backend abstracts an image builder whose only purpose is to build an image referenced by an imageID.
type Backend interface {
	// Build builds a Docker image referenced by an imageID string.
	//
	// Note: Tagging an image should not be done by a Builder, it should instead be done
	// by the caller.
	//
	// TODO: make this return a reference instead of string
	Build() (imageID string)
}
