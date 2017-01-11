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
	assert.Equal(t, req.Source, "app-secret")
	assert.Equal(t, req.Target, "app-secret")
	assert.Equal(t, req.UID, "0")
	assert.Equal(t, req.GID, "0")
}

func TestSecretOptionsSourceTarget(t *testing.T) {
	var opt SecretOpt

	testCase := "source=foo,target=testing"
	assert.NilError(t, opt.Set(testCase))

	reqs := opt.Value()
	assert.Equal(t, len(reqs), 1)
	req := reqs[0]
	assert.Equal(t, req.Source, "foo")
	assert.Equal(t, req.Target, "testing")
}

func TestSecretOptionsCustomUidGid(t *testing.T) {
	var opt SecretOpt

	testCase := "source=foo,target=testing,uid=1000,gid=1001"
	assert.NilError(t, opt.Set(testCase))

	reqs := opt.Value()
	assert.Equal(t, len(reqs), 1)
	req := reqs[0]
	assert.Equal(t, req.Source, "foo")
	assert.Equal(t, req.Target, "testing")
	assert.Equal(t, req.UID, "1000")
	assert.Equal(t, req.GID, "1001")
}

func TestSecretOptionsCustomMode(t *testing.T) {
	var opt SecretOpt

	testCase := "source=foo,target=testing,uid=1000,gid=1001,mode=0444"
	assert.NilError(t, opt.Set(testCase))

	reqs := opt.Value()
	assert.Equal(t, len(reqs), 1)
	req := reqs[0]
	assert.Equal(t, req.Source, "foo")
	assert.Equal(t, req.Target, "testing")
	assert.Equal(t, req.UID, "1000")
	assert.Equal(t, req.GID, "1001")
	assert.Equal(t, req.Mode, os.FileMode(0444))
}
