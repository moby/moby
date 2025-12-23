package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/v2/daemon/server/buildbackend"
	"github.com/moby/moby/v2/daemon/server/httputils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type mockBuildBackend struct {
	pruneOpts   buildbackend.CachePruneOptions
	pruneReport *build.CachePruneReport
	pruneErr    error
}

func (m *mockBuildBackend) PruneCache(ctx context.Context, opts buildbackend.CachePruneOptions) (*build.CachePruneReport, error) {
	m.pruneOpts = opts
	if m.pruneErr != nil {
		return nil, m.pruneErr
	}
	if m.pruneReport != nil {
		return m.pruneReport, nil
	}
	return &build.CachePruneReport{
		SpaceReclaimed: 12345,
		CachesDeleted:  []string{"cache1", "cache2"},
	}, nil
}

func (m *mockBuildBackend) Build(ctx context.Context, config buildbackend.BuildConfig) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockBuildBackend) Cancel(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

func TestPostPrune(t *testing.T) {
	testcases := []struct {
		name        string
		apiVersion  string
		requestBody string
		queryParams url.Values
		validate    func(t *testing.T, opts buildbackend.CachePruneOptions)
		expError    string
	}{
		{
			name:       "API v1.53 JSON body all parameters",
			apiVersion: "1.53",
			requestBody: `{
				"all": true,
				"reservedSpace": 1024,
				"maxUsedSpace": 2048,
				"minFreeSpace": 512
			}`,
			validate: func(t *testing.T, opts buildbackend.CachePruneOptions) {
				assert.Check(t, is.Equal(opts.All, true))
				assert.Check(t, is.Equal(opts.ReservedSpace, int64(1024)))
				assert.Check(t, is.Equal(opts.MaxUsedSpace, int64(2048)))
				assert.Check(t, is.Equal(opts.MinFreeSpace, int64(512)))
			},
		},
		{
			name:       "API v1.53 JSON body with filters",
			apiVersion: "1.53",
			requestBody: `{
				"all":false,
				"reservedSpace":2048,
				"maxUsedSpace":4096,
				"filters":{"dangling":{"true":true},"until":{"24h": true},
				"label":{"env=prod":true,"tier=backend":true}}
			}`,
			validate: func(t *testing.T, opts buildbackend.CachePruneOptions) {
				assert.Check(t, is.Equal(opts.All, false))
				assert.Check(t, opts.Filters.Contains("until"))
				assert.Check(t, opts.Filters.Contains("label"))
			},
		},
		{
			name:       "API v1.53 invalid JSON",
			apiVersion: "1.53",
			requestBody: `{
				"all": true,
				"reservedSpace": "not a number"
			}`,
			expError: "error parsing JSON body",
		},
		{
			name:       "API v1.53 malformed JSON",
			apiVersion: "1.53",
			requestBody: `{
				"all": true,
				"reservedSpace": 1024
			`, // missing closing brace
			expError: "error parsing JSON body",
		},
		{
			name:       "API v1.52 query args all parameters",
			apiVersion: "1.52",
			queryParams: url.Values{
				"all":            []string{"1"},
				"reserved-space": []string{"1024"},
				"max-used-space": []string{"2048"},
				"min-free-space": []string{"512"},
			},
			validate: func(t *testing.T, opts buildbackend.CachePruneOptions) {
				assert.Check(t, is.Equal(opts.All, true))
				assert.Check(t, is.Equal(opts.ReservedSpace, int64(1024)))
				assert.Check(t, is.Equal(opts.MaxUsedSpace, int64(2048)))
				assert.Check(t, is.Equal(opts.MinFreeSpace, int64(512)))
			},
		},
		{
			name:       "API v1.52 query args all=false",
			apiVersion: "1.52",
			queryParams: url.Values{
				"all": []string{"0"},
			},
			validate: func(t *testing.T, opts buildbackend.CachePruneOptions) {
				assert.Check(t, is.Equal(opts.All, false))
			},
		},
		{
			name:       "API v1.52 with filters",
			apiVersion: "1.52",
			queryParams: url.Values{
				"filters": []string{`{"until":["24h"]}`},
			},
			validate: func(t *testing.T, opts buildbackend.CachePruneOptions) {
				assert.Check(t, opts.Filters.Contains("until"))
			},
		},
		{
			name:       "API v1.52 invalid reserved-space",
			apiVersion: "1.52",
			queryParams: url.Values{
				"reserved-space": []string{"not-a-number"},
			},
			expError: "reserved-space is in bytes and expects an integer",
		},
		{
			name:       "API v1.52 invalid max-used-space",
			apiVersion: "1.52",
			queryParams: url.Values{
				"max-used-space": []string{"invalid"},
			},
			expError: "max-used-space is in bytes and expects an integer",
		},
		{
			name:       "API v1.52 invalid min-free-space",
			apiVersion: "1.52",
			queryParams: url.Values{
				"min-free-space": []string{"bad-value"},
			},
			expError: "min-free-space is in bytes and expects an integer",
		},
		{
			name:       "API v1.52 invalid filters JSON",
			apiVersion: "1.52",
			queryParams: url.Values{
				"filters": []string{`{invalid json}`},
			},
			expError: "invalid filter",
		},
		{
			name:       "API v1.52 zero values not sent",
			apiVersion: "1.52",
			queryParams: url.Values{
				"all": []string{"0"},
			},
			validate: func(t *testing.T, opts buildbackend.CachePruneOptions) {
				assert.Check(t, is.Equal(opts.All, false))
				assert.Check(t, is.Equal(opts.ReservedSpace, int64(0)))
				assert.Check(t, is.Equal(opts.MaxUsedSpace, int64(0)))
				assert.Check(t, is.Equal(opts.MinFreeSpace, int64(0)))
			},
		},
		{
			name:        "API v1.53 empty JSON body",
			apiVersion:  "1.53",
			requestBody: `{}`,
			validate: func(t *testing.T, opts buildbackend.CachePruneOptions) {
				assert.Check(t, is.Equal(opts.All, false))
				assert.Check(t, is.Equal(opts.ReservedSpace, int64(0)))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			backend := &mockBuildBackend{}
			router := &buildRouter{backend: backend}

			// Create request
			var req *http.Request
			if tc.requestBody != "" {
				req = httptest.NewRequest(http.MethodPost, "/build/prune", bytes.NewReader([]byte(tc.requestBody)))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(http.MethodPost, "/build/prune", nil)
			}

			// Add query parameters
			if tc.queryParams != nil {
				req.URL.RawQuery = tc.queryParams.Encode()
			}

			// Set API version in context
			ctx := context.WithValue(context.Background(), httputils.APIVersionKey{}, tc.apiVersion)
			req = req.WithContext(ctx)

			// Parse form (required by postPrune)
			if err := req.ParseForm(); err != nil {
				t.Fatalf("failed to parse form: %v", err)
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Call postPrune
			err := router.postPrune(ctx, w, req, nil)

			// Check error expectations
			if tc.expError != "" {
				assert.Check(t, err != nil, "expected error but got nil")
				assert.Check(t, is.ErrorContains(err, tc.expError))
				return
			}

			// No error expected
			assert.NilError(t, err)

			// Validate the options passed to backend
			if tc.validate != nil {
				tc.validate(t, backend.pruneOpts)
			}

			// Check response
			assert.Check(t, is.Equal(w.Code, http.StatusOK))

			var report build.CachePruneReport
			err = json.NewDecoder(w.Body).Decode(&report)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(report.SpaceReclaimed, uint64(12345)))
			assert.Check(t, is.DeepEqual(report.CachesDeleted, []string{"cache1", "cache2"}))
		})
	}
}

func TestPostPruneBackendError(t *testing.T) {
	backend := &mockBuildBackend{
		pruneErr: fmt.Errorf("backend error: cache prune failed"),
	}
	router := &buildRouter{backend: backend}

	req := httptest.NewRequest(http.MethodPost, "/build/prune", bytes.NewReader([]byte(`{"all":true}`)))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(context.Background(), httputils.APIVersionKey{}, "1.53")
	req = req.WithContext(ctx)

	if err := req.ParseForm(); err != nil {
		t.Fatalf("failed to parse form: %v", err)
	}

	w := httptest.NewRecorder()

	err := router.postPrune(ctx, w, req, nil)
	assert.Check(t, err != nil)
	assert.Check(t, is.ErrorContains(err, "cache prune failed"))
}

func TestPostPruneVersionBoundary(t *testing.T) {
	// Test the exact version boundary (v1.53) between query args and JSON body
	testcases := []struct {
		name         string
		apiVersion   string
		usesJSONBody bool
		requestBody  string
		queryParams  url.Values
	}{
		{
			name:         "v1.52 uses query args (< v1.53)",
			apiVersion:   "1.52",
			usesJSONBody: false,
			queryParams: url.Values{
				"all": []string{"1"},
			},
		},
		{
			name:         "v1.53 uses JSON body (>= v1.53)",
			apiVersion:   "1.53",
			usesJSONBody: true,
			requestBody:  `{"all":true}`,
		},
		{
			name:         "v1.54 uses JSON body (>= v1.53)",
			apiVersion:   "1.54",
			usesJSONBody: true,
			requestBody:  `{"all":true}`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			backend := &mockBuildBackend{}
			router := &buildRouter{backend: backend}

			var req *http.Request
			if tc.usesJSONBody {
				req = httptest.NewRequest(http.MethodPost, "/build/prune", bytes.NewReader([]byte(tc.requestBody)))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(http.MethodPost, "/build/prune", nil)
				if tc.queryParams != nil {
					req.URL.RawQuery = tc.queryParams.Encode()
				}
			}

			ctx := context.WithValue(context.Background(), httputils.APIVersionKey{}, tc.apiVersion)
			req = req.WithContext(ctx)

			if err := req.ParseForm(); err != nil {
				t.Fatalf("failed to parse form: %v", err)
			}

			w := httptest.NewRecorder()

			err := router.postPrune(ctx, w, req, nil)
			assert.NilError(t, err)

			// Verify All parameter was correctly parsed regardless of method
			assert.Check(t, is.Equal(backend.pruneOpts.All, true))
		})
	}
}
