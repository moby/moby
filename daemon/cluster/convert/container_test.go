package convert // import "github.com/docker/docker/daemon/cluster/convert"

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestTmpfsOptionsToGRPC(t *testing.T) {
	options := [][]string{
		{"noexec"},
		{"uid", "12345"},
	}

	expected := `[["noexec"],["uid","12345"]]`
	actual := tmpfsOptionsToGRPC(options)
	assert.Equal(t, expected, actual)
}

func TestTmpfsOptionsFromGRPC(t *testing.T) {
	options := `[["noexec"],["uid","12345"]]`

	expected := [][]string{
		{"noexec"},
		{"uid", "12345"},
	}
	actual := tmpfsOptionsFromGRPC(options)

	assert.DeepEqual(t, expected, actual)
}
