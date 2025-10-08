//go:build !windows

package system

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestInfoBinaryCommits(t *testing.T) {
	ctx := setupTest(t)

	// API v1.48 and lower returned both the "current" commit (ID) and "expected" commit.
	// The "Expected" field has been removed in the API types, so define
	// an ad-hoc type for this test.
	type legacyCommit struct {
		ID       string
		Expected string
	}

	type legacyInfo struct {
		ContainerdCommit legacyCommit
		RuncCommit       legacyCommit
		InitCommit       legacyCommit
	}

	t.Run("current", func(t *testing.T) {
		res, body, err := request.Get(ctx, "/info", request.JSON)
		assert.NilError(t, err)
		assert.Equal(t, res.StatusCode, http.StatusOK)

		buf, err := request.ReadBody(body)
		assert.NilError(t, err)

		var info legacyInfo
		err = json.Unmarshal(buf, &info)
		assert.NilError(t, err)

		assert.Check(t, info.ContainerdCommit.ID != "N/A")
		assert.Check(t, is.Equal(info.ContainerdCommit.Expected, ""))

		assert.Check(t, info.InitCommit.ID != "N/A")
		assert.Check(t, is.Equal(info.InitCommit.Expected, ""))

		assert.Check(t, info.RuncCommit.ID != "N/A")
		assert.Check(t, is.Equal(info.RuncCommit.Expected, ""))
	})

	// Expected commits are omitted in API 1.49, but should still be included in older versions.
	t.Run("1.48", func(t *testing.T) {
		res, body, err := request.Get(ctx, "/v1.48/info", request.JSON)
		assert.NilError(t, err)
		assert.Equal(t, res.StatusCode, http.StatusOK)

		buf, err := request.ReadBody(body)
		assert.NilError(t, err)

		var info legacyInfo
		err = json.Unmarshal(buf, &info)
		assert.NilError(t, err)

		assert.Check(t, info.ContainerdCommit.ID != "N/A")
		assert.Check(t, is.Equal(info.ContainerdCommit.Expected, info.ContainerdCommit.ID))

		assert.Check(t, info.InitCommit.ID != "N/A")
		assert.Check(t, is.Equal(info.InitCommit.Expected, info.InitCommit.ID))

		assert.Check(t, info.RuncCommit.ID != "N/A")
		assert.Check(t, is.Equal(info.RuncCommit.Expected, info.RuncCommit.ID))
	})
}

func TestInfoLegacyFields(t *testing.T) {
	ctx := setupTest(t)

	const notPresent = "expected field to not be present"

	tests := []struct {
		name           string
		url            string
		expectedFields map[string]any
	}{
		{
			name: "api v1.49 legacy bridge-nftables",
			url:  "/v1.49/info",
			expectedFields: map[string]any{
				"BridgeNfIp6tables": false,
				"BridgeNfIptables":  false,
			},
		},
		{
			name: "api v1.50 legacy bridge-nftables",
			url:  "/v1.50/info",
			expectedFields: map[string]any{
				"BridgeNfIp6tables": notPresent,
				"BridgeNfIptables":  notPresent,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := request.Get(ctx, tc.url)
			assert.NilError(t, err)
			assert.Equal(t, res.StatusCode, http.StatusOK)
			body, err := io.ReadAll(res.Body)
			assert.NilError(t, err)

			actual := map[string]any{}
			err = json.Unmarshal(body, &actual)
			assert.NilError(t, err, string(body))

			for field, expectedValue := range tc.expectedFields {
				if expectedValue == notPresent {
					_, found := actual[field]
					assert.Assert(t, !found, "field %s should not be present", field)
				} else {
					_, found := actual[field]
					assert.Assert(t, found, "field %s should be present", field)
					assert.Check(t, is.DeepEqual(actual[field], expectedValue))
				}
			}
		})
	}
}
