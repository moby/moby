
package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"strings"

	"github.com/moby/moby/api/types/build"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestBuildCachePrune(t *testing.T) {
	expectedReport := build.CachePruneReport{
		SpaceReclaimed: 12345,
		CachesDeleted:  []string{"cache1", "cache2"},
	}

	tests := []struct {
		name            string
		version         string
		opts            BuildCachePruneOptions
		expectJSONBody  bool // true => expect JSON body, false => expect query args
		validateReq     func(t *testing.T, req *http.Request)
	}{
		{
			name:    "API v1.52 should use query args (< v1.53)",
			version: "1.52",
			opts: BuildCachePruneOptions{
				All:           true,
				ReservedSpace: 1024,
				MaxUsedSpace:  2048,
				MinFreeSpace:  512,
				Filters: Filters{
					"dangling": {
						"true": true,
					},
					"label": {
						"env=prod": true,
						"tier=backend": true,
					},
				},
			},
			expectJSONBody: false, // v1.52 < v1.53, should use query args
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Check(t, is.Equal(req.Method, http.MethodPost))
				assert.Check(t, is.Equal(req.URL.Path, "/v1.52/build/prune"))
				// BUG: Current code uses JSON body for v1.52 (should use query args)
				body, err := io.ReadAll(req.Body)
				
				// ensure there is no JSON body
				assert.NilError(t, err)
				assert.Check(t, len(body) == 0, "BUG: v1.52 should use query args, not JSON body")

				// ensure query args used properly
				assert.Check(t, is.Equal(req.URL.Query().Get("all"), "1"))
				assert.Check(t, is.Equal(req.URL.Query().Get("reserved-space"), "1024"))
				assert.Check(t, is.Equal(req.URL.Query().Get("max-used-space"), "2048"))
				assert.Check(t, is.Equal(req.URL.Query().Get("min-free-space"), "512"))

				assert.Check(t, strings.Contains(req.URL.Query().Get("filters"), `"label"`), `Filters must contain "label"`)
			},
		},
		{
			name:    "API v1.53 should use JSON body (>= v1.53)",
			version: "1.53",
			opts: BuildCachePruneOptions{
				All:           false,
				ReservedSpace: 2048,
				MaxUsedSpace:  4096,
				MinFreeSpace:  0,
				Filters: Filters{
					"dangling": {
						"true": true,
					},
					"label": {
						"env=prod": true,
						"tier=backend": true,
					},
				},
			},
			expectJSONBody: true,  // v1.53 >= v1.53, should use JSON body
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Check(t, is.Equal(req.Method, http.MethodPost))
				assert.Check(t, is.Equal(req.URL.Path, "/v1.53/build/prune"))
				
				// ensure v1.53 uses JSON body
				body, err := io.ReadAll(req.Body)
				assert.NilError(t, err)
				assert.Check(t, len(body) > 0, "v1.53 should use JSON body")
				
				var pruneReq BuildCachePruneRequest
				err = json.Unmarshal(body, &pruneReq)
				assert.NilError(t, err)
				
				// BUG: opts.All=false but hardcoded to true
				assert.Check(t, is.Equal(pruneReq.All, false))
				assert.Check(t, is.Equal(pruneReq.ReservedSpace, int64(2048)))
				assert.Check(t, is.Equal(pruneReq.MaxUsedSpace, int64(4096)))
				assert.Check(t, is.Equal(pruneReq.MinFreeSpace, int64(0)))

				_, ok := pruneReq.Filters["label"]
				assert.Check(t, ok, `"Filters must contain "label"`)

				qry := req.URL.Query()

				_, all_err := qry["all"]
				_, rs_err := qry["reserved-space"]
				_, mus_err := qry["max-used-space"]
				_, mfs_err := qry["min-free-space"]
				assert.Check(t, !all_err, `"all" must not exist in query args`)
				assert.Check(t, !rs_err, `"reserved-space" must not exist in query args`)
				assert.Check(t, !mus_err, `"max-used-space" must not exist in query args`)
				assert.Check(t, !mfs_err, `"min-free-space" must not exist in query args`)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, err := New(
				WithMockClient(func(req *http.Request) (*http.Response, error) {
					// Validate the request
					tc.validateReq(t, req)
					
					// Return mock response
					responseBody, err := json.Marshal(expectedReport)
					if err != nil {
						return nil, err
					}
					return mockResponse(http.StatusOK, nil, string(responseBody))(req)
				}),
				WithAPIVersion(tc.version),
			)
			assert.NilError(t, err)

			result, err := client.BuildCachePrune(context.Background(), tc.opts)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(result.Report.SpaceReclaimed, expectedReport.SpaceReclaimed))
			assert.Check(t, is.DeepEqual(result.Report.CachesDeleted, expectedReport.CachesDeleted))
		})
	}
}