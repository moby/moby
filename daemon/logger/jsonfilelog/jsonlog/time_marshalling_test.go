package jsonlog // import "github.com/docker/docker/daemon/logger/jsonfilelog/jsonlog"

import (
	"testing"
	"time"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestFastTimeMarshalJSONWithInvalidYear(t *testing.T) {
	aTime := time.Date(-1, 1, 1, 0, 0, 0, 0, time.Local)
	_, err := fastTimeMarshalJSON(aTime)
	assert.Check(t, is.ErrorContains(err, "year outside of range"))

	anotherTime := time.Date(10000, 1, 1, 0, 0, 0, 0, time.Local)
	_, err = fastTimeMarshalJSON(anotherTime)
	assert.Check(t, is.ErrorContains(err, "year outside of range"))
}

func TestFastTimeMarshalJSON(t *testing.T) {
	aTime := time.Date(2015, 5, 29, 11, 1, 2, 3, time.UTC)
	json, err := fastTimeMarshalJSON(aTime)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("\"2015-05-29T11:01:02.000000003Z\"", json))

	location, err := time.LoadLocation("Europe/Paris")
	assert.NilError(t, err)

	aTime = time.Date(2015, 5, 29, 11, 1, 2, 3, location)
	json, err = fastTimeMarshalJSON(aTime)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("\"2015-05-29T11:01:02.000000003+02:00\"", json))
}
