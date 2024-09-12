package opts

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSetOpts(t *testing.T) {
	tmpMap := make(map[string]bool)
	o := NewSetOpts(tmpMap)
	assert.NilError(t, o.Set("feature-a=1"))
	assert.NilError(t, o.Set("feature-b=true"))
	assert.NilError(t, o.Set("feature-c=0"))
	assert.NilError(t, o.Set("feature-d=false"))

	expected := "map[feature-a:true feature-b:true feature-c:false feature-d:false]"
	assert.Check(t, is.Equal(expected, o.String()))

	expectedValue := map[string]bool{"feature-a": true, "feature-b": true, "feature-c": false, "feature-d": false}
	assert.Check(t, is.DeepEqual(expectedValue, o.GetAll()))

	err := o.Set("feature=not-a-bool")
	assert.Check(t, is.Error(err, `strconv.ParseBool: parsing "not-a-bool": invalid syntax`))
}

func TestNamedSetOpts(t *testing.T) {
	tmpMap := make(map[string]bool)
	o := NewNamedSetOpts("features", tmpMap)
	assert.Check(t, is.Equal("features", o.Name()))

	assert.NilError(t, o.Set("feature-a=1"))
	assert.NilError(t, o.Set("feature-b=true"))
	assert.NilError(t, o.Set("feature-c=0"))
	assert.NilError(t, o.Set("feature-d=false"))

	expected := "map[feature-a:true feature-b:true feature-c:false feature-d:false]"
	assert.Check(t, is.Equal(expected, o.String()))

	expectedValue := map[string]bool{"feature-a": true, "feature-b": true, "feature-c": false, "feature-d": false}
	assert.Check(t, is.DeepEqual(expectedValue, o.GetAll()))

	err := o.Set("feature=not-a-bool")
	assert.Check(t, is.Error(err, `strconv.ParseBool: parsing "not-a-bool": invalid syntax`))
}
