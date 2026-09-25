package portallocator

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseReservedPorts(t *testing.T) {
	tests := []struct {
		name        string
		list        string
		begin, end  int
		expected    map[uint16]struct{}
		expectedErr string
	}{
		{
			name: "empty",
			list: "\n",
		},
		{
			name:     "single port",
			list:     "8080\n",
			expected: map[uint16]struct{}{8080: {}},
		},
		{
			name:     "multiple ports",
			list:     "8080,9148",
			expected: map[uint16]struct{}{8080: {}, 9148: {}},
		},
		{
			name:     "port range",
			list:     "30000-30002",
			expected: map[uint16]struct{}{30000: {}, 30001: {}, 30002: {}},
		},
		{
			name:     "mixed",
			list:     "8080,30000-30001,9148",
			expected: map[uint16]struct{}{8080: {}, 30000: {}, 30001: {}, 9148: {}},
		},
		{
			name:  "port outside allocation range",
			list:  "8080,30000",
			begin: 20000,
			end:   40000,
			// 8080 is below the allocation range, the allocator never picks it.
			expected: map[uint16]struct{}{30000: {}},
		},
		{
			name:     "range clamped to allocation range",
			list:     "29998-30001",
			begin:    30000,
			end:      40000,
			expected: map[uint16]struct{}{30000: {}, 30001: {}},
		},
		{
			name:     "range fully outside allocation range",
			list:     "8080-8082",
			begin:    30000,
			end:      40000,
			expected: map[uint16]struct{}{},
		},
		{
			name:        "invalid port",
			list:        "8080,abc",
			expectedErr: `invalid port "abc"`,
		},
		{
			name:        "invalid range",
			list:        "30002-30000",
			expectedErr: `invalid port range "30002-30000"`,
		},
		{
			name:        "incomplete range",
			list:        "30000-",
			expectedErr: `invalid port range "30000-"`,
		},
		{
			name:        "out of range port",
			list:        "65536",
			expectedErr: `invalid port "65536"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			begin, end := tc.begin, tc.end
			if begin == 0 && end == 0 {
				begin, end = 1, 65535
			}
			reserved, err := parseReservedPorts(tc.list, begin, end)
			if tc.expectedErr != "" {
				assert.Check(t, is.ErrorContains(err, tc.expectedErr))
				return
			}
			assert.Check(t, err)
			assert.Check(t, is.DeepEqual(reserved, tc.expected))
		})
	}
}
