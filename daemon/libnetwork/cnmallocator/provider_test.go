package cnmallocator

import (
	"testing"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/testutils"
	"google.golang.org/grpc/codes"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestValidateDriver(t *testing.T) {
	p := NewProvider(nil)

	for _, tt := range []struct {
		name      string
		validator func(*api.Driver) error
	}{
		{"IPAM", p.ValidateIPAMDriver},
		{"Network", p.ValidateNetworkDriver},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, tt.validator(nil))

			err := tt.validator(&api.Driver{Name: ""})
			assert.Check(t, is.ErrorContains(err, ""))
			assert.Check(t, is.Equal(codes.InvalidArgument, testutils.ErrorCode(err)))
		})
	}
}
