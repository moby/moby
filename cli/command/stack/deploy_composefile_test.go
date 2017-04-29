package stack

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/pkg/testutil/tempfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigDetails(t *testing.T) {
	content := `
version: "3.0"
services:
  foo:
    image: alpine:3.5
`
	file := tempfile.NewTempFile(t, "test-get-config-details", content)
	defer file.Remove()

	details, err := getConfigDetails(file.Name())
	require.NoError(t, err)
	assert.Equal(t, filepath.Dir(file.Name()), details.WorkingDir)
	assert.Len(t, details.ConfigFiles, 1)
	assert.Len(t, details.Environment, len(os.Environ()))
}
