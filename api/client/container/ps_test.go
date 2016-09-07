package container

import "github.com/go-check/check"

func (s *DockerSuite) TestBuildContainerListOptions(c *check.C) {

	contexts := []struct {
		psOpts          *psOptions
		expectedAll     bool
		expectedSize    bool
		expectedLimit   int
		expectedFilters map[string]string
	}{
		{
			psOpts: &psOptions{
				all:    true,
				size:   true,
				last:   5,
				filter: []string{"foo=bar", "baz=foo"},
			},
			expectedAll:   true,
			expectedSize:  true,
			expectedLimit: 5,
			expectedFilters: map[string]string{
				"foo": "bar",
				"baz": "foo",
			},
		},
		{
			psOpts: &psOptions{
				all:     true,
				size:    true,
				last:    -1,
				nLatest: true,
			},
			expectedAll:     true,
			expectedSize:    true,
			expectedLimit:   1,
			expectedFilters: make(map[string]string),
		},
	}

	for _, ctxs := range contexts {
		options, err := buildContainerListOptions(ctxs.psOpts)
		if err != nil {
			c.Fatal(err)
		}

		if ctxs.expectedAll != options.All {
			c.Fatalf("Expected All to be %t but got %t", ctxs.expectedAll, options.All)
		}

		if ctxs.expectedSize != options.Size {
			c.Fatalf("Expected Size to be %t but got %t", ctxs.expectedSize, options.Size)
		}

		if ctxs.expectedLimit != options.Limit {
			c.Fatalf("Expected Limit to be %d but got %d", ctxs.expectedLimit, options.Limit)
		}

		f := options.Filter

		if f.Len() != len(ctxs.expectedFilters) {
			c.Fatalf("Expected %d filters but got %d", len(ctxs.expectedFilters), f.Len())
		}

		for k, v := range ctxs.expectedFilters {
			f := options.Filter
			if !f.ExactMatch(k, v) {
				c.Fatalf("Expected filter with key %s to be %s but got %s", k, v, f.Get(k))
			}
		}
	}
}
