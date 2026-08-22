package libnetwork

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestEndpointJoinInfoUnmarshalJSON(t *testing.T) {
	t.Run("non-bool disableGatewayService returns an error", func(t *testing.T) {
		data, err := json.Marshal(map[string]any{
			"disableGatewayService": "not-a-bool",
		})
		assert.NilError(t, err)

		var epj endpointJoinInfo
		assert.Check(t, epj.UnmarshalJSON(data) != nil)
	})

	t.Run("non-string gw is logged and ignored, not an error", func(t *testing.T) {
		data, err := json.Marshal(map[string]any{
			"gw":                    123,
			"disableGatewayService": false,
		})
		assert.NilError(t, err)

		var epj endpointJoinInfo
		assert.NilError(t, epj.UnmarshalJSON(data))
		assert.Check(t, is.Nil(epj.gw))
	})
}
