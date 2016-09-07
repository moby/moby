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
		c.Assert(err, check.IsNil)

		c.Assert(options.All, check.Equals, ctxs.expectedAll)
		c.Assert(options.Size, check.Equals, ctxs.expectedSize)
		c.Assert(options.Limit, check.Equals, ctxs.expectedLimit)

		f := options.Filter
		c.Assert(f.Len(), check.Equals, len(ctxs.expectedFilters))

		for k, v := range ctxs.expectedFilters {
			f := options.Filter
			if !f.ExactMatch(k, v) {
				c.Fatalf("Expected filter with key %s to be %s but got %s", k, v, f.Get(k))
			}
		}
	}
}
