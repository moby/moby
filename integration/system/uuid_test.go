package system

import (
	"context"
	"testing"

	"github.com/pborman/uuid"
	"gotest.tools/assert"
)

func TestUUIDGeneration(t *testing.T) {
	defer setupTest(t)()

	c := testEnv.APIClient()
	info, err := c.Info(context.Background())
	assert.NilError(t, err)

	id := uuid.Parse(info.ID)
	assert.Equal(t, id != nil, true)
}
