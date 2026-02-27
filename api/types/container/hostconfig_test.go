package container

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestValidateRestartPolicy(t *testing.T) {
	tests := []struct {
		name        string
		input       RestartPolicy
		expectedErr string
	}{
		{
			name:  "empty",
			input: RestartPolicy{},
		},
		{
			name:        "empty with invalid MaxRestartCount (for backward compatibility)",
			input:       RestartPolicy{MaximumRetryCount: 123},
			expectedErr: "", // Allowed for backward compatibility
		},
		{
			name:        "empty with negative MaxRestartCount)",
			input:       RestartPolicy{MaximumRetryCount: -123},
			expectedErr: "", // Allowed for backward compatibility
		},
		{
			name:  "always",
			input: RestartPolicy{Name: RestartPolicyAlways},
		},
		{
			name:        "always with MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyAlways, MaximumRetryCount: 123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure'",
		},
		{
			name:        "always with negative MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyAlways, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure' and cannot be negative",
		},
		{
			name:  "unless-stopped",
			input: RestartPolicy{Name: RestartPolicyUnlessStopped},
		},
		{
			name:        "unless-stopped with MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyUnlessStopped, MaximumRetryCount: 123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure'",
		},
		{
			name:        "unless-stopped with negative MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyUnlessStopped, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure' and cannot be negative",
		},
		{
			name:  "disabled",
			input: RestartPolicy{Name: RestartPolicyDisabled},
		},
		{
			name:        "disabled with MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyDisabled, MaximumRetryCount: 123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure'",
		},
		{
			name:        "disabled with negative MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyDisabled, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure' and cannot be negative",
		},
		{
			name:  "on-failure",
			input: RestartPolicy{Name: RestartPolicyOnFailure},
		},
		{
			name:  "on-failure with MaxRestartCount",
			input: RestartPolicy{Name: RestartPolicyOnFailure, MaximumRetryCount: 123},
		},
		{
			name:        "on-failure with negative MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyOnFailure, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count cannot be negative",
		},
		{
			name:        "unknown policy",
			input:       RestartPolicy{Name: "unknown"},
			expectedErr: "invalid restart policy: unknown policy 'unknown'; use one of 'no', 'always', 'on-failure', or 'unless-stopped'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRestartPolicy(tc.input)
			if tc.expectedErr == "" {
				assert.Check(t, err)
			} else {
				assert.Check(t, is.ErrorType(err, isInvalidParameter))
				assert.Check(t, is.Error(err, tc.expectedErr))
			}
		})
	}
}

// isInvalidParameter is a minimal implementation of [github.com/containerd/errdefs.IsInvalidArgument],
// because this was the only import of that package in api/types, which is the
// package imported by external users.
func isInvalidParameter(err error) bool {
	_, ok := err.(interface {
		InvalidParameter()
	})
	return ok
}

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
			expectedOK:  true,
			description: "should accept 'container' without colon and return empty ID",
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
			assert.Check(t, is.Equal(ok, tc.expectedOK), "containerID(%q) ok = %v, expected %v: %s",
				tc.input, ok, tc.expectedOK, tc.description)
			assert.Check(t, is.Equal(id, tc.expectedID), "containerID(%q) id = %q, expected %q: %s",
				tc.input, id, tc.expectedID, tc.description)
		})
	}
}
