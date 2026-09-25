package image

// Identity holds information about the identity and origin of the image.
// This is trusted information verified by the daemon and cannot be modified
// by tagging an image to a different name.
type Identity struct {
	// Signature contains the properties of verified signatures for the image.
	Signature []SignatureIdentity `json:"Signature,omitzero"`
	// Pull contains remote location information if image was created via pull.
	// If image was pulled via mirror, this contains the original repository location.
	// After successful push this images also contains the pushed repository location.
	Pull []PullIdentity `json:"Pull,omitzero"`
	// Build contains build reference information if image was created via build.
	Build []BuildIdentity `json:"Build,omitzero"`
}
