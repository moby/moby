package daemon

import (
	"testing"

	"gotest.tools/v3/assert"
)

type mockVendorLister struct {
	vendors []string
}

func (m *mockVendorLister) ListVendors() []string {
	return m.vendors
}

func TestDiscoverGPUVendor(t *testing.T) {
	tests := []struct {
		name         string
		vendors      []string
		expectVendor string
		expectError  string
	}{
		{
			name:        "Nil vendors",
			vendors:     nil,
			expectError: "vendor lister not available",
		},
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
			vendors:     []string{},
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
			var lister vendorLister
			if tt.vendors != nil {
				lister = &mockVendorLister{vendors: tt.vendors}
			}
			vendor, err := discoverGPUVendor(lister)

			if tt.expectError != "" {
				assert.Error(t, err, tt.expectError)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, tt.expectVendor, vendor)
			}
		})
	}
}
