package opts

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretOptionsSimple(t *testing.T) {
	var opt SecretOpt

	testCase := "app-secret"
	assert.NoError(t, opt.Set(testCase))

	reqs := opt.Value()
	require.Len(t, reqs, 1)
	req := reqs[0]
	assert.Equal(t, "app-secret", req.SecretName)
	assert.Equal(t, "app-secret", req.File.Name)
	assert.Equal(t, "0", req.File.UID)
	assert.Equal(t, "0", req.File.GID)
}

func TestSecretOptionsSourceTarget(t *testing.T) {
	var opt SecretOpt

	testCase := "source=foo,target=testing"
	assert.NoError(t, opt.Set(testCase))

	reqs := opt.Value()
	require.Len(t, reqs, 1)
	req := reqs[0]
	assert.Equal(t, "foo", req.SecretName)
	assert.Equal(t, "testing", req.File.Name)
}

func TestSecretOptionsShorthand(t *testing.T) {
	var opt SecretOpt

	testCase := "src=foo,target=testing"
	assert.NoError(t, opt.Set(testCase))

	reqs := opt.Value()
	require.Len(t, reqs, 1)
	req := reqs[0]
	assert.Equal(t, "foo", req.SecretName)
}

func TestSecretOptionsCustomUidGid(t *testing.T) {
	var opt SecretOpt

	testCase := "source=foo,target=testing,uid=1000,gid=1001"
	assert.NoError(t, opt.Set(testCase))

	reqs := opt.Value()
	require.Len(t, reqs, 1)
	req := reqs[0]
	assert.Equal(t, "foo", req.SecretName)
	assert.Equal(t, "testing", req.File.Name)
	assert.Equal(t, "1000", req.File.UID)
	assert.Equal(t, "1001", req.File.GID)
}

func TestSecretOptionsCustomMode(t *testing.T) {
	var opt SecretOpt

	testCase := "source=foo,target=testing,uid=1000,gid=1001,mode=0444"
	assert.NoError(t, opt.Set(testCase))

	reqs := opt.Value()
	require.Len(t, reqs, 1)
	req := reqs[0]
	assert.Equal(t, "foo", req.SecretName)
	assert.Equal(t, "testing", req.File.Name)
	assert.Equal(t, "1000", req.File.UID)
	assert.Equal(t, "1001", req.File.GID)
	assert.Equal(t, os.FileMode(0444), req.File.Mode)
}
