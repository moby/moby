package daemon

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetFirstAvailableVendor(t *testing.T) {
	tests := []struct {
		name         string
		vendors      []string
		expectVendor string
		expectError  string
	}{
		{
			name:         "NVIDIA vendor",
			vendors:      []string{"nvidia.com"},
			expectVendor: "nvidia.com",
		},
		{
			name:         "AMD vendor",
			vendors:      []string{"amd.com"},
			expectVendor: "amd.com",
		},
		{
			name:        "No vendors",
			vendors:     nil,
			expectError: "no known GPU vendor found",
		},
		{
			name:        "Unknown vendor",
			vendors:     []string{"unknown.com"},
			expectError: "no known GPU vendor found",
		},
		{
			name:         "Mixed vendor",
			vendors:      []string{"amd.com", "nvidia.com"},
			expectVendor: "nvidia.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vendor, err := getFirstAvailableVendor(tt.vendors)

			if tt.expectError != "" {
				assert.Error(t, err, tt.expectError)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, tt.expectVendor, vendor)
			}
		})
	}
}
