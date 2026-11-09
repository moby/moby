package build

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moby/moby/api/types/jsonstream"
	"gotest.tools/v3/assert"
)

func TestFormatBuildError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "regular error",
			err:  errors.New("build failed"),
			want: `{"error":"build failed","errorDetail":{"message":"build failed"}}`,
		},
		{
			name: "container exit code",
			err:  &jsonstream.Error{Code: 42, Message: "command failed"},
			want: `{"error":"command failed","errorDetail":{"code":42,"message":"command failed"}}`,
		},
		{
			name: "wrapped JSON error",
			err:  fmt.Errorf("wrapped: %w", &jsonstream.Error{Code: 42, Message: "command failed"}),
			want: `{"error":"command failed","errorDetail":{"code":42,"message":"command failed"}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatBuildError(tc.err)
			assert.DeepEqual(t, string(got), tc.want+"\r\n")
		})
	}
}
