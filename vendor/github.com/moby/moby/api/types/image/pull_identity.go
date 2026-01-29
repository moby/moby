package image

// PullIdentity contains remote location information if image was created via pull.
// If image was pulled via mirror, this contains the original repository location.
type PullIdentity struct {
	// Repository is the remote repository location the image was pulled from.
	Repository string `json:"Repository,omitempty"`
}
