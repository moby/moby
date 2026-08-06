package libnetwork

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestIpamInfoUnmarshalJSON(t *testing.T) {
	t.Run("malformed Meta is ignored", func(t *testing.T) {
		data, err := json.Marshal(map[string]any{
			"PoolID":   "pool1",
			"Meta":     []string{"not", "a", "map"},
			"IPAMData": `{"AddressSpace":"as1"}`,
		})
		assert.NilError(t, err)

		var i IpamInfo
		assert.NilError(t, i.UnmarshalJSON(data))
		assert.Check(t, is.Equal(i.PoolID, "pool1"))
		assert.Check(t, is.Nil(i.Meta))
	})

	t.Run("malformed IPAMData returns an error", func(t *testing.T) {
		data, err := json.Marshal(map[string]any{
			"PoolID":   "pool1",
			"IPAMData": "not valid json",
		})
		assert.NilError(t, err)

		var i IpamInfo
		assert.Check(t, i.UnmarshalJSON(data) != nil)
	})
}