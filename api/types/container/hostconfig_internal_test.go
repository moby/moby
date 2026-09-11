package container

import "testing"

func TestContainerID(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedID  string
		expectedOK  bool
		description string
	}{
		{
			name:        "container without colon",
			input:       "container",
			expectedID:  "",
			expectedOK:  false,
			description: "should reject 'container' without a colon; container-mode requires 'container:<name|id>'",
		},
		{
			name:        "container with empty ID",
			input:       "container:",
			expectedID:  "",
			expectedOK:  true,
			description: "should accept 'container:' with empty ID",
		},
		{
			name:        "container with valid ID",
			input:       "container:abc123",
			expectedID:  "abc123",
			expectedOK:  true,
			description: "should extract container ID from 'container:abc123'",
		},
		{
			name:        "container with full container ID",
			input:       "container:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expectedID:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expectedOK:  true,
			description: "should extract full 64-character container ID",
		},
		{
			name:        "container with name",
			input:       "container:my-container-name",
			expectedID:  "my-container-name",
			expectedOK:  true,
			description: "should extract container name",
		},
		{
			name:        "container with colon in ID",
			input:       "container:foo:bar",
			expectedID:  "foo:bar",
			expectedOK:  true,
			description: "should handle colons in container ID/name",
		},
		{
			name:        "host network mode",
			input:       "host",
			expectedID:  "",
			expectedOK:  false,
			description: "should reject 'host' network mode",
		},
		{
			name:        "bridge network mode",
			input:       "bridge",
			expectedID:  "",
			expectedOK:  false,
			description: "should reject 'bridge' network mode",
		},
		{
			name:        "empty string",
			input:       "",
			expectedID:  "",
			expectedOK:  false,
			description: "should reject empty string",
		},
		{
			name:        "containerX prefix",
			input:       "containerX",
			expectedID:  "",
			expectedOK:  false,
			description: "should reject strings that start with 'container' but don't match exactly",
		},
		{
			name:        "Xcontainer suffix",
			input:       "Xcontainer",
			expectedID:  "",
			expectedOK:  false,
			description: "should reject strings that end with 'container'",
		},
		{
			name:        "custom network name",
			input:       "mynetwork",
			expectedID:  "",
			expectedOK:  false,
			description: "should reject custom network names",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := containerID(tc.input)
			if ok != tc.expectedOK {
				t.Errorf("containerID(%q) ok = %v, want %v: %s", tc.input, ok, tc.expectedOK, tc.description)
			}
			if id != tc.expectedID {
				t.Errorf("containerID(%q) id = %q, want %q: %s", tc.input, id, tc.expectedID, tc.description)
			}
		})
	}
}
