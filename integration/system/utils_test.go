package system

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/common"
	"gotest.tools/v3/assert"
)

// getErrorMessage returns the error message from an error API response
func getErrorMessage(t *testing.T, body []byte) string {
	t.Helper()
	var resp common.ErrorResponse
	assert.NilError(t, json.Unmarshal(body, &resp))
	return strings.TrimSpace(resp.Message)
}
