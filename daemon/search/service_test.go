package search // import "github.com/docker/docker/daemon/search"

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

type fakeService struct {
	shouldReturnError bool

	term    string
	results []registry.SearchResult
}

func (s *fakeService) Search(ctx context.Context, term string, limit int, authConfig *registry.AuthConfig, userAgent string, headers map[string][]string) (*registry.SearchResults, error) {
	if s.shouldReturnError {
		return nil, errdefs.Unknown(errors.New("search unknown error"))
	}
	return &registry.SearchResults{
		Query:      s.term,
		NumResults: len(s.results),
		Results:    s.results,
	}, nil
}

func TestSearchImagesErrors(t *testing.T) {
	errorCases := []struct {
		filtersArgs       filters.Args
		shouldReturnError bool
		expectedError     string
	}{
		{
			expectedError:     "search unknown error",
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
			service := &Service{
				registrySearch: &fakeService{
					shouldReturnError: tc.shouldReturnError,
				},
			}
			_, err := service.SearchImages(context.Background(), "term", registry.SearchOpts{
				Filters: tc.filtersArgs,
			})
			assert.ErrorContains(t, err, tc.expectedError)
			if tc.shouldReturnError {
				assert.Check(t, errdefs.IsUnknown(err), "got: %T: %v", err, err)
				return
			}
			assert.Check(t, errdefs.IsInvalidParameter(err), "got: %T: %v", err, err)
		})
	}
}

func TestSearchImages(t *testing.T) {
	term := "term"
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
					IsAutomated: true,
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsAutomated: true,
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
					IsAutomated: true,
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
					IsAutomated: false,
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsAutomated: false,
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
					IsAutomated: true,
				},
				{
					Name:        "name1",
					Description: "description1",
					StarCount:   1,
					IsOfficial:  false,
					IsAutomated: true,
				},
				{
					Name:        "name2",
					Description: "description2",
					StarCount:   1,
					IsOfficial:  true,
					IsAutomated: false,
				},
				{
					Name:        "name3",
					Description: "description3",
					StarCount:   2,
					IsOfficial:  true,
					IsAutomated: true,
				},
			},
			expectedResults: []registry.SearchResult{
				{
					Name:        "name3",
					Description: "description3",
					StarCount:   2,
					IsOfficial:  true,
					IsAutomated: true,
				},
			},
		},
	}
	for _, tc := range successCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service := &Service{
				registrySearch: &fakeService{
					term:    term,
					results: tc.registryResults,
				},
			}
			results, err := service.SearchImages(context.Background(), term, registry.SearchOpts{
				Filters: tc.filtersArgs,
			})
			assert.NilError(t, err)
			assert.Equal(t, results.Query, term)
			assert.Equal(t, results.NumResults, len(tc.expectedResults))
			assert.DeepEqual(t, results.Results, tc.expectedResults)
		})
	}
}
