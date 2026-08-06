package libnetwork

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestIpamInfoUnmarshalJSON(t *testing.T) {
	t.Run("Meta with the wrong top-level type is discarded", func(t *testing.T) {
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

	t.Run("Meta with one malformed value is discarded, not partially kept", func(t *testing.T) {
		// encoding/json would otherwise decode the "gateway" key fine and
		// only zero out "weird", silently leaving a partially-valid map.
		data, err := json.Marshal(map[string]any{
			"PoolID":   "pool1",
			"Meta":     map[string]any{"gateway": "10.0.0.1", "weird": 123},
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
