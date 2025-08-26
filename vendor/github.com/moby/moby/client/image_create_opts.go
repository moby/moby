package client

// ImageCreateOptions holds information to create images.
type ImageCreateOptions struct {
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry.
	Platform     string // Platform is the target platform of the image if it needs to be pulled from the registry.
}
