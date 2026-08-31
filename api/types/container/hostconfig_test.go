package container_test

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestValidateRestartPolicy(t *testing.T) {
	tests := []struct {
		name        string
		input       container.RestartPolicy
		expectedErr string
	}{
		{
			name:  "empty",
			input: container.RestartPolicy{},
		},
		{
			name:        "empty with invalid MaxRestartCount (for backward compatibility)",
			input:       container.RestartPolicy{MaximumRetryCount: 123},
			expectedErr: "", // Allowed for backward compatibility
		},
		{
			name:        "empty with negative MaxRestartCount)",
			input:       container.RestartPolicy{MaximumRetryCount: -123},
			expectedErr: "", // Allowed for backward compatibility
		},
		{
			name:  "always",
			input: container.RestartPolicy{Name: container.RestartPolicyAlways},
		},
		{
			name:        "always with MaxRestartCount",
			input:       container.RestartPolicy{Name: container.RestartPolicyAlways, MaximumRetryCount: 123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure'",
		},
		{
			name:        "always with negative MaxRestartCount",
			input:       container.RestartPolicy{Name: container.RestartPolicyAlways, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure' and cannot be negative",
		},
		{
			name:  "unless-stopped",
			input: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
		},
		{
			name:        "unless-stopped with MaxRestartCount",
			input:       container.RestartPolicy{Name: container.RestartPolicyUnlessStopped, MaximumRetryCount: 123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure'",
		},
		{
			name:        "unless-stopped with negative MaxRestartCount",
			input:       container.RestartPolicy{Name: container.RestartPolicyUnlessStopped, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure' and cannot be negative",
		},
		{
			name:  "disabled",
			input: container.RestartPolicy{Name: container.RestartPolicyDisabled},
		},
		{
			name:        "disabled with MaxRestartCount",
			input:       container.RestartPolicy{Name: container.RestartPolicyDisabled, MaximumRetryCount: 123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure'",
		},
		{
			name:        "disabled with negative MaxRestartCount",
			input:       container.RestartPolicy{Name: container.RestartPolicyDisabled, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count can only be used with 'on-failure' and cannot be negative",
		},
		{
			name:  "on-failure",
			input: container.RestartPolicy{Name: container.RestartPolicyOnFailure},
		},
		{
			name:  "on-failure with MaxRestartCount",
			input: container.RestartPolicy{Name: container.RestartPolicyOnFailure, MaximumRetryCount: 123},
		},
		{
			name:        "on-failure with negative MaxRestartCount",
			input:       container.RestartPolicy{Name: container.RestartPolicyOnFailure, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count cannot be negative",
		},
		{
			name:        "unknown policy",
			input:       container.RestartPolicy{Name: "unknown"},
			expectedErr: "invalid restart policy: unknown policy 'unknown'; use one of 'no', 'always', 'on-failure', or 'unless-stopped'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := container.ValidateRestartPolicy(tc.input)
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
