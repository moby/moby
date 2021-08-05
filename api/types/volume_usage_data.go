package types

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

// VolumeUsageData Data usage details about the volume. This information is used by the
// `GET /system/df` and `GET /volumes/usage` endpoint, and omitted in other endpoints.
//
// swagger:model VolumeUsageData
type VolumeUsageData struct {

	// The number of containers referencing this volume. This field
	// is set to `-1` if the reference-count is not available.
	//
	// Required: true
	RefCount int64 `json:"RefCount"`

	// Amount of disk space used by the volume (in bytes). This information
	// is only available for volumes created with the `"local"` volume
	// driver. For volumes created with other volume drivers, this field
	// is set to `-1` ("not available")
	//
	// Required: true
	Size int64 `json:"Size"`
}
