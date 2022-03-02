package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/registry"
)

type fakeService struct {
	registry.Service
	shouldReturnError bool

	term    string
	results []registrytypes.SearchResult
}

func (s *fakeService) Search(ctx context.Context, term string, limit int, authConfig *types.AuthConfig, userAgent string, headers map[string][]string) (*registrytypes.SearchResults, error) {
	if s.shouldReturnError {
		return nil, errdefs.Unknown(errors.New("search unknown error"))
	}
	return &registrytypes.SearchResults{
		Query:      s.term,
		NumResults: len(s.results),
		Results:    s.results,
	}, nil
}

func TestSearchRegistryForImagesErrors(t *testing.T) {
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
	for index, e := range errorCases {
		daemon := &ImageService{
			registryService: &fakeService{
				shouldReturnError: e.shouldReturnError,
			},
		}
		_, err := daemon.SearchRegistryForImages(context.Background(), e.filtersArgs, "term", 0, nil, map[string][]string{})
		if err == nil {
			t.Errorf("%d: expected an error, got nothing", index)
		}
		if !strings.Contains(err.Error(), e.expectedError) {
			t.Errorf("%d: expected error to contain %s, got %s", index, e.expectedError, err.Error())
		}
		if e.shouldReturnError {
			if !errdefs.IsUnknown(err) {
				t.Errorf("%d: expected expected an errdefs.ErrUnknown, got: %T: %v", index, err, err)
			}
			continue
		}
		if !errdefs.IsInvalidParameter(err) {
			t.Errorf("%d: expected expected an errdefs.ErrInvalidParameter, got: %T: %v", index, err, err)
		}
	}
}

func TestSearchRegistryForImages(t *testing.T) {
	term := "term"
	successCases := []struct {
		filtersArgs     filters.Args
		registryResults []registrytypes.SearchResult
		expectedResults []registrytypes.SearchResult
	}{
		{
			registryResults: []registrytypes.SearchResult{},
			expectedResults: []registrytypes.SearchResult{},
		},
		{
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
			expectedResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("is-automated", "true")),
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
			expectedResults: []registrytypes.SearchResult{},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("is-automated", "true")),
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsAutomated: true,
				},
			},
			expectedResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsAutomated: true,
				},
			},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("is-automated", "false")),
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsAutomated: true,
				},
			},
			expectedResults: []registrytypes.SearchResult{},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("is-automated", "false")),
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsAutomated: false,
				},
			},
			expectedResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsAutomated: false,
				},
			},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("is-official", "true")),
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
			expectedResults: []registrytypes.SearchResult{},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("is-official", "true")),
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsOfficial:  true,
				},
			},
			expectedResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsOfficial:  true,
				},
			},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("is-official", "false")),
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsOfficial:  true,
				},
			},
			expectedResults: []registrytypes.SearchResult{},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("is-official", "false")),
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsOfficial:  false,
				},
			},
			expectedResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					IsOfficial:  false,
				},
			},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("stars", "0")),
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					StarCount:   0,
				},
			},
			expectedResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					StarCount:   0,
				},
			},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("stars", "1")),
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
					StarCount:   0,
				},
			},
			expectedResults: []registrytypes.SearchResult{},
		},
		{
			filtersArgs: filters.NewArgs(filters.Arg("stars", "1")),
			registryResults: []registrytypes.SearchResult{
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
			expectedResults: []registrytypes.SearchResult{
				{
					Name:        "name1",
					Description: "description1",
					StarCount:   1,
				},
			},
		},
		{
			filtersArgs: filters.NewArgs(
				filters.Arg("stars", "1"),
				filters.Arg("is-official", "true"),
				filters.Arg("is-automated", "true"),
			),
			registryResults: []registrytypes.SearchResult{
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
			expectedResults: []registrytypes.SearchResult{
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
	for index, s := range successCases {
		daemon := &ImageService{
			registryService: &fakeService{
				term:    term,
				results: s.registryResults,
			},
		}
		results, err := daemon.SearchRegistryForImages(context.Background(), s.filtersArgs, term, 0, nil, map[string][]string{})
		if err != nil {
			t.Errorf("%d: %v", index, err)
		}
		if results.Query != term {
			t.Errorf("%d: expected Query to be %s, got %s", index, term, results.Query)
		}
		if results.NumResults != len(s.expectedResults) {
			t.Errorf("%d: expected NumResults to be %d, got %d", index, len(s.expectedResults), results.NumResults)
		}
		for _, result := range results.Results {
			found := false
			for _, expectedResult := range s.expectedResults {
				if expectedResult.Name == result.Name &&
					expectedResult.Description == result.Description &&
					expectedResult.IsAutomated == result.IsAutomated &&
					expectedResult.IsOfficial == result.IsOfficial &&
					expectedResult.StarCount == result.StarCount {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%d: expected results %v, got %v", index, s.expectedResults, results.Results)
			}
		}
	}
}
