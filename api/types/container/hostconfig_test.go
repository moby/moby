package container

import (
	"testing"

	"github.com/docker/docker/errdefs"
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
			expectedErr: "invalid restart policy: maximum retry count cannot be used with restart policy 'always'",
		},
		{
			name:        "always with negative MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyAlways, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count cannot be used with restart policy 'always'",
		},
		{
			name:  "unless-stopped",
			input: RestartPolicy{Name: RestartPolicyUnlessStopped},
		},
		{
			name:        "unless-stopped with MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyUnlessStopped, MaximumRetryCount: 123},
			expectedErr: "invalid restart policy: maximum retry count cannot be used with restart policy 'unless-stopped'",
		},
		{
			name:        "unless-stopped with negative MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyUnlessStopped, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count cannot be used with restart policy 'unless-stopped'",
		},
		{
			name:  "disabled",
			input: RestartPolicy{Name: RestartPolicyDisabled},
		},
		{
			name:        "disabled with MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyDisabled, MaximumRetryCount: 123},
			expectedErr: "invalid restart policy: maximum retry count cannot be used with restart policy 'no'",
		},
		{
			name:        "disabled with negative MaxRestartCount",
			input:       RestartPolicy{Name: RestartPolicyDisabled, MaximumRetryCount: -123},
			expectedErr: "invalid restart policy: maximum retry count cannot be used with restart policy 'no'",
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
			input:       RestartPolicy{Name: "I do not exist"},
			expectedErr: "invalid restart policy: 'I do not exist'",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRestartPolicy(tc.input)
			if tc.expectedErr == "" {
				assert.Check(t, err)
			} else {
				assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
				assert.Check(t, is.Error(err, tc.expectedErr))
			}
		})
	}
}
