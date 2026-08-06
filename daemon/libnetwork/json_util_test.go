package libnetwork

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestUnmarshalJSONField(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
	}

	t.Run("happy path", func(t *testing.T) {
		m := map[string]any{"field": map[string]any{"name": "foo"}}
		var dst sample
		err := unmarshalJSONField(m, "field", &dst)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(dst, sample{Name: "foo"}))
	})

	t.Run("slice destination", func(t *testing.T) {
		m := map[string]any{"field": []any{"a", "b", "c"}}
		var dst []string
		err := unmarshalJSONField(m, "field", &dst)
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(dst, []string{"a", "b", "c"}))
	})

	t.Run("missing key succeeds and leaves the destination unchanged", func(t *testing.T) {
		m := map[string]any{}
		dst := sample{Name: "preexisting"}
		err := unmarshalJSONField(m, "field", &dst)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(dst, sample{Name: "preexisting"}))
	})

	t.Run("array where struct is expected returns an error", func(t *testing.T) {
		m := map[string]any{"field": []any{"unexpected", "array"}}
		var dst sample
		err := unmarshalJSONField(m, "field", &dst)
		assert.Check(t, err != nil)
	})
}
