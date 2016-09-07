package daemon

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/types"
	registrytypes "github.com/docker/engine-api/types/registry"
	"github.com/go-check/check"
)

type FakeService struct {
	registry.DefaultService

	shouldReturnError bool

	term    string
	results []registrytypes.SearchResult
}

func (s *FakeService) Search(ctx context.Context, term string, limit int, authConfig *types.AuthConfig, userAgent string, headers map[string][]string) (*registrytypes.SearchResults, error) {
	if s.shouldReturnError {
		return nil, fmt.Errorf("Search unknown error")
	}
	return &registrytypes.SearchResults{
		Query:      s.term,
		NumResults: len(s.results),
		Results:    s.results,
	}, nil
}

func (s *DockerSuite) TestSearchRegistryForImagesErrors(c *check.C) {
	errorCases := []struct {
		filtersArgs       string
		shouldReturnError bool
		expectedError     string
	}{
		{
			expectedError:     "Search unknown error",
			shouldReturnError: true,
		},
		{
			filtersArgs:   "invalid json",
			expectedError: "invalid character 'i' looking for beginning of value",
		},
		{
			filtersArgs:   `{"type":{"custom":true}}`,
			expectedError: "Invalid filter 'type'",
		},
		{
			filtersArgs:   `{"is-automated":{"invalid":true}}`,
			expectedError: "Invalid filter 'is-automated=[invalid]'",
		},
		{
			filtersArgs:   `{"is-automated":{"true":true,"false":true}}`,
			expectedError: "Invalid filter 'is-automated",
		},
		{
			filtersArgs:   `{"is-official":{"invalid":true}}`,
			expectedError: "Invalid filter 'is-official=[invalid]'",
		},
		{
			filtersArgs:   `{"is-official":{"true":true,"false":true}}`,
			expectedError: "Invalid filter 'is-official",
		},
		{
			filtersArgs:   `{"stars":{"invalid":true}}`,
			expectedError: "Invalid filter 'stars=invalid'",
		},
		{
			filtersArgs:   `{"stars":{"1":true,"invalid":true}}`,
			expectedError: "Invalid filter 'stars=invalid'",
		},
	}
	for index, e := range errorCases {
		daemon := &Daemon{
			RegistryService: &FakeService{
				shouldReturnError: e.shouldReturnError,
			},
		}
		_, err := daemon.SearchRegistryForImages(context.Background(), e.filtersArgs, "term", 25, nil, map[string][]string{})
		if err == nil {
			c.Errorf("%d: expected an error, got nothing", index)
		}
		if !strings.Contains(err.Error(), e.expectedError) {
			c.Errorf("%d: expected error to contain %s, got %s", index, e.expectedError, err.Error())
		}
	}
}

func (s *DockerSuite) TestSearchRegistryForImages(c *check.C) {
	term := "term"
	successCases := []struct {
		filtersArgs     string
		registryResults []registrytypes.SearchResult
		expectedResults []registrytypes.SearchResult
	}{
		{
			filtersArgs:     "",
			registryResults: []registrytypes.SearchResult{},
			expectedResults: []registrytypes.SearchResult{},
		},
		{
			filtersArgs: "",
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
			filtersArgs: `{"is-automated":{"true":true}}`,
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
			expectedResults: []registrytypes.SearchResult{},
		},
		{
			filtersArgs: `{"is-automated":{"true":true}}`,
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
			filtersArgs: `{"is-automated":{"false":true}}`,
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
			filtersArgs: `{"is-automated":{"false":true}}`,
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
			filtersArgs: `{"is-official":{"true":true}}`,
			registryResults: []registrytypes.SearchResult{
				{
					Name:        "name",
					Description: "description",
				},
			},
			expectedResults: []registrytypes.SearchResult{},
		},
		{
			filtersArgs: `{"is-official":{"true":true}}`,
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
			filtersArgs: `{"is-official":{"false":true}}`,
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
			filtersArgs: `{"is-official":{"false":true}}`,
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
			filtersArgs: `{"stars":{"0":true}}`,
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
			filtersArgs: `{"stars":{"1":true}}`,
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
			filtersArgs: `{"stars":{"1":true}}`,
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
			filtersArgs: `{"stars":{"1":true}, "is-official":{"true":true}, "is-automated":{"true":true}}`,
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
	for index, sc := range successCases {
		daemon := &Daemon{
			RegistryService: &FakeService{
				term:    term,
				results: sc.registryResults,
			},
		}
		results, err := daemon.SearchRegistryForImages(context.Background(), sc.filtersArgs, term, 25, nil, map[string][]string{})
		if err != nil {
			c.Errorf("%d: %v", index, err)
		}
		if results.Query != term {
			c.Errorf("%d: expected Query to be %s, got %s", index, term, results.Query)
		}
		if results.NumResults != len(sc.expectedResults) {
			c.Errorf("%d: expected NumResults to be %d, got %d", index, len(sc.expectedResults), results.NumResults)
		}
		for _, result := range results.Results {
			found := false
			for _, expectedResult := range sc.expectedResults {
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
				c.Errorf("%d: expected results %v, got %v", index, sc.expectedResults, results.Results)
			}
		}
	}
}
