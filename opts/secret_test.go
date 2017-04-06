package opts

import (
	"os"
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

func TestSecretOptionsSimple(t *testing.T) {
	var opt SecretOpt

	testCase := "app-secret"
	assert.NilError(t, opt.Set(testCase))

	reqs := opt.Value()
	assert.Equal(t, len(reqs), 1)
	req := reqs[0]
	assert.Equal(t, req.SecretName, "app-secret")
	assert.Equal(t, req.File.Name, "app-secret")
	assert.Equal(t, req.File.UID, "0")
	assert.Equal(t, req.File.GID, "0")
}

func TestSecretOptionsSourceTarget(t *testing.T) {
	var opt SecretOpt

	testCase := "source=foo,target=testing"
	assert.NilError(t, opt.Set(testCase))

	reqs := opt.Value()
	assert.Equal(t, len(reqs), 1)
	req := reqs[0]
	assert.Equal(t, req.SecretName, "foo")
	assert.Equal(t, req.File.Name, "testing")
}

func TestSecretOptionsShorthand(t *testing.T) {
	var opt SecretOpt

	testCase := "src=foo,target=testing"
	assert.NilError(t, opt.Set(testCase))

	reqs := opt.Value()
	assert.Equal(t, len(reqs), 1)
	req := reqs[0]
	assert.Equal(t, req.SecretName, "foo")
}

func TestSecretOptionsCustomUidGid(t *testing.T) {
	var opt SecretOpt

	testCase := "source=foo,target=testing,uid=1000,gid=1001"
	assert.NilError(t, opt.Set(testCase))

	reqs := opt.Value()
	assert.Equal(t, len(reqs), 1)
	req := reqs[0]
	assert.Equal(t, req.SecretName, "foo")
	assert.Equal(t, req.File.Name, "testing")
	assert.Equal(t, req.File.UID, "1000")
	assert.Equal(t, req.File.GID, "1001")
}

func TestSecretOptionsCustomMode(t *testing.T) {
	var opt SecretOpt

	testCase := "source=foo,target=testing,uid=1000,gid=1001,mode=0444"
	assert.NilError(t, opt.Set(testCase))

	reqs := opt.Value()
	assert.Equal(t, len(reqs), 1)
	req := reqs[0]
	assert.Equal(t, req.SecretName, "foo")
	assert.Equal(t, req.File.Name, "testing")
	assert.Equal(t, req.File.UID, "1000")
	assert.Equal(t, req.File.GID, "1001")
	assert.Equal(t, req.File.Mode, os.FileMode(0444))
}
