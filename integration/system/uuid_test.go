package system

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/assert"
)

func TestUUIDGeneration(t *testing.T) {
	defer setupTest(t)()

	c := testEnv.APIClient()
	info, err := c.Info(context.Background())
	assert.NilError(t, err)

	_, err = uuid.Parse(info.ID)
	assert.NilError(t, err)
}
