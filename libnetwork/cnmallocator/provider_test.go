package cnmallocator

import (
	"testing"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/testutils"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
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
			assert.NoError(t, tt.validator(nil))

			err := tt.validator(&api.Driver{Name: ""})
			assert.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, testutils.ErrorCode(err))
		})
	}
}
