package daemon

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestCDIDeviceInjectorResolveVendor(t *testing.T) {
	testCases := []struct {
		description      string
		availableVendors mockVendorLister
		allowedVendors   []string

		expectedError  string
		expectedVendor string
	}{
		{
			description:      "predefined vendors not found",
			availableVendors: []string{"not.amd.com", "not.nvidia.com"},
			expectedError:    "no known GPU vendor found",
		},
		{
			description:      "required vendor is used",
			availableVendors: []string{"another.com", "example.com"},
			allowedVendors:   []string{"example.com"},
			expectedVendor:   "example.com",
		},
		{
			description:      "required vendor not found",
			availableVendors: []string{"another.com", "example.com"},
			allowedVendors:   []string{"not.example.com"},
			expectedError:    "no known GPU vendor found",
		},
		{
			description:      "multiple required / allowed vendors",
			availableVendors: []string{"another.com", "example.com"},
			allowedVendors:   []string{"not.example.com", "another.com"},
			expectedVendor:   "another.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			i := createCDIDeviceInjector(tc.availableVendors, tc.allowedVendors...)

			vendor, err := i.resolveVendor()
			if tc.expectedError == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectedError)
			}
			assert.Equal(t, tc.expectedVendor, vendor)
		})
	}
}

type mockVendorLister []string

func (m mockVendorLister) ListVendors() []string {
	return m
}
