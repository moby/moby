package image

// SignatureIdentity contains the properties of verified signatures for the image.
type SignatureIdentity struct {
	// Name is a textual description summarizing the type of signature.
	Name string `json:"Name,omitempty"`
	// Timestamps contains a list of verified signed timestamps for the signature.
	Timestamps []SignatureTimestamp `json:"Timestamps,omitzero"`
	// KnownSigner is an identifier for a special signer identity that is known to the implementation.
	KnownSigner KnownSignerIdentity `json:"KnownSigner,omitempty"`
	// DockerReference is the Docker image reference associated with the signature.
	// This is an optional field only present in older hashedrecord signatures.
	DockerReference string `json:"DockerReference,omitempty"`
	// Signer contains information about the signer certificate used to sign the image.
	Signer *SignerIdentity `json:"Signer,omitempty"`
	// SignatureType is the type of signature format. E.g. "bundle-v0.3" or "hashedrecord".
	SignatureType SignatureType `json:"SignatureType,omitempty"`

	// Error contains error information if signature verification failed.
	// Other fields will be empty in this case.
	Error string `json:"Error,omitempty"`
	// Warnings contains any warnings that occurred during signature verification.
	// For example, if there was no internet connectivity and cached trust roots were used.
	// Warning does not indicate a failed verification but may point to configuration issues.
	Warnings []string `json:"Warnings,omitzero"`
}
