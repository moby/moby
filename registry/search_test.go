package registry // import "github.com/docker/docker/registry"

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestSearchErrors(t *testing.T) {
	errorCases := []struct {
		filtersArgs       filters.Args
		shouldReturnError bool
		expectedError     string
	}{
		{
			expectedError:     "Unexpected status code 500",
			shouldReturnError: true,
		},
		{
			filtersArgs:   filters.NewArgs(filters.Arg("type", "custom")),
			expectedError: "invalid filter 'type'",
		},
		{
			filtersArgs:   filters.NewArgs(filters.Arg("is-automated", "invalid")),
			expectedError: "invalid filter 'is-automated=[invalid]'",
		},
		{
			filtersArgs: filters.NewArgs(
				filters.Arg("is-automated", "true"),
				filters.Arg("is-automated", "false"),
			),
			expectedError: "invalid filter 'is-automated",
		},
		{
			filtersArgs:   filters.NewArgs(filters.Arg("is-official", "invalid")),
			expectedError: "invalid filter 'is-official=[invalid]'",
		},
		{
			filtersArgs: filters.NewArgs(
				filters.Arg("is-official", "true"),
				filters.Arg("is-official", "false"),
			),
			expectedError: "invalid filter 'is-official",
		},
		{
			filtersArgs:   filters.NewArgs(filters.Arg("stars", "invalid")),
			expectedError: "invalid filter 'stars=invalid'",
		},
		{
			filtersArgs: filters.NewArgs(
				filters.Arg("stars", "1"),
				filters.Arg("stars", "invalid"),
			),
			expectedError: "invalid filter 'stars=invalid'",
		},
	}
	for _, tc := range errorCases {
		tc := tc
		t.Run(tc.expectedError, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !tc.shouldReturnError {
					t.Errorf("unexpected HTTP request")
				}
				http.Error(w, "no search for you", http.StatusInternalServerError)
			}))
			defer srv.Close()

			// Construct the search term by cutting the 'http://' prefix off srv.URL.
			term := srv.URL[7:] + "/term"

			reg, err := NewService(ServiceOptions{})
			assert.NilError(t, err)
			_, err = reg.Search(context.Background(), tc.filtersArgs, term, 0, nil, map[string][]string{})
			assert.ErrorContains(t, err, tc.expectedError)
			if tc.shouldReturnError {
				assert.Check(t, errdefs.IsUnknown(err), "got: %T: %v", err, err)
				return
			}
			assert.Check(t, errdefs.IsInvalidParameter(err), "got: %T: %v", err, err)
		})
	}
}

func TestSearch(t *testing.T) {
	const term = "term"
	successCases := []struct {
		name            string
		filtersArgs     filters.Args
		registryResults []registry.SearchResult
		expectedResults []registry.SearchResult
	}{
		{
			name:            "empty results",
			registryResults: []registry.SearchResult{},
			expectedResults: []registry.SearchResult{},
		},
		{
			name: "no filter",
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
		},
		{
			name:        "is-automated=true, no results",
			filtersArgs: filters.NewArgs(filters.Arg("is-automated", "true")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
			expectedResults: []registry.SearchResult{},
		},
		{
			name:        "is-automated=true",
			filtersArgs: filters.NewArgs(filters.Arg("is-automated", "true")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsAutomated: true, //nolint:staticcheck // ignore SA1019 (field is deprecated).
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsAutomated: true, //nolint:staticcheck // ignore SA1019 (field is deprecated).
				},
			},
		},
		{
			name:        "is-automated=false, no results",
			filtersArgs: filters.NewArgs(filters.Arg("is-automated", "false")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsAutomated: true, //nolint:staticcheck // ignore SA1019 (field is deprecated).
				},
			},
			expectedResults: []registry.SearchResult{},
		},
		{
			name:        "is-automated=false",
			filtersArgs: filters.NewArgs(filters.Arg("is-automated", "false")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
		},
		{
			name:        "is-official=true, no results",
			filtersArgs: filters.NewArgs(filters.Arg("is-official", "true")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
			expectedResults: []registry.SearchResult{},
		},
		{
			name:        "is-official=true",
			filtersArgs: filters.NewArgs(filters.Arg("is-official", "true")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsOfficial:  true,
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsOfficial:  true,
				},
			},
		},
		{
			name:        "is-official=false, no results",
			filtersArgs: filters.NewArgs(filters.Arg("is-official", "false")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsOfficial:  true,
				},
			},
			expectedResults: []registry.SearchResult{},
		},
		{
			name:        "is-official=false",
			filtersArgs: filters.NewArgs(filters.Arg("is-official", "false")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsOfficial:  false,
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsOfficial:  false,
				},
			},
		},
		{
			name:        "stars=0",
			filtersArgs: filters.NewArgs(filters.Arg("stars", "0")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					StarCount:   0,
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					StarCount:   0,
				},
			},
		},
		{
			name:        "stars=0, no results",
			filtersArgs: filters.NewArgs(filters.Arg("stars", "1")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					StarCount:   0,
				},
			},
			expectedResults: []registry.SearchResult{},
		},
		{
			name:        "stars=1",
			filtersArgs: filters.NewArgs(filters.Arg("stars", "1")),
			registryResults: []registry.SearchResult{
				{
					Name:        "name0",
					Description: "description0",
					StarCount:   0,
				},
				{
					Name:        "name1",
					Description: "description1",
					StarCount:   1,
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name1",
					Description: "description1",
					StarCount:   1,
				},
			},
		},
		{
			name: "stars=1, is-official=true, is-automated=true",
			filtersArgs: filters.NewArgs(
				filters.Arg("stars", "1"),
				filters.Arg("is-official", "true"),
				filters.Arg("is-automated", "true"),
			),
			registryResults: []registry.SearchResult{
				{
					Name:        "name0",
					Description: "description0",
					StarCount:   0,
					IsOfficial:  true,
					IsAutomated: true, //nolint:staticcheck // ignore SA1019 (field is deprecated).
				},
				{
					Name:        "name1",
					Description: "description1",
					StarCount:   1,
					IsOfficial:  false,
					IsAutomated: true, //nolint:staticcheck // ignore SA1019 (field is deprecated).
				},
				{
					Name:        "name2",
					Description: "description2",
					StarCount:   1,
					IsOfficial:  true,
				},
				{
					Name:        "name3",
					Description: "description3",
					StarCount:   2,
					IsOfficial:  true,
					IsAutomated: true, //nolint:staticcheck // ignore SA1019 (field is deprecated).
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name3",
					Description: "description3",
					StarCount:   2,
					IsOfficial:  true,
					IsAutomated: true, //nolint:staticcheck // ignore SA1019 (field is deprecated).
				},
			},
		},
	}
	for _, tc := range successCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-type", "application/json")
				json.NewEncoder(w).Encode(registry.SearchResults{
					Query:      term,
					NumResults: len(tc.registryResults),
					Results:    tc.registryResults,
				})
			}))
			defer srv.Close()

			// Construct the search term by cutting the 'http://' prefix off srv.URL.
			searchTerm := srv.URL[7:] + "/" + term

			reg, err := NewService(ServiceOptions{})
			assert.NilError(t, err)
			results, err := reg.Search(context.Background(), tc.filtersArgs, searchTerm, 0, nil, map[string][]string{})
			assert.NilError(t, err)
			assert.DeepEqual(t, results, tc.expectedResults)
		})
	}
}
