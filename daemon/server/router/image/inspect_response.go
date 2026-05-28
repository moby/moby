package image

// legacyConfigFields defines legacy image-config fields to include in
// API responses on older API versions.
var legacyConfigFields = map[string]map[string]any{
	// Legacy fields for API v1.49 and lower. These fields are deprecated
	// and omitted in newer API versions; see https://github.com/moby/moby/pull/48457
	"v1.49": {
		"AttachStderr": false,
		"AttachStdin":  false,
		"AttachStdout": false,
		"Cmd":          nil,
		"Domainname":   "",
		"Entrypoint":   nil,
		"Env":          nil,
		"Hostname":     "",
		"Image":        "",
		"Labels":       nil,
		"OnBuild":      nil,
		"OpenStdin":    false,
		"StdinOnce":    false,
		"Tty":          false,
		"User":         "",
		"Volumes":      nil,
		"WorkingDir":   "",
	},
	// Legacy fields for API v1.50 and v1.51. These fields did not have
	// an "omitempty" and were always included in the response, even if
	// not set; see https://github.com/moby/moby/issues/50134
	"v1.50-v1.51": {
		"Cmd":        nil,
		"Entrypoint": nil,
		"Env":        nil,
		"Labels":     nil,
		"OnBuild":    nil,
		"User":       "",
		"Volumes":    nil,
		"WorkingDir": "",
	},
}
