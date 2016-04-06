// +build !windows

package image

func getOSVersion() string {
	// For Linux, images do not specify a version.
	return ""
}

func hasOSFeature(_ string) bool {
	// Linux currently has no OS features
	return false
}
