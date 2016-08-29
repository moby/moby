package container

import "testing"

func TestBuildContainerListOptions(t *testing.T) {

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

	for _, c := range contexts {
		options, err := buildContainerListOptions(c.psOpts)
		if err != nil {
			t.Fatal(err)
		}

		if c.expectedAll != options.All {
			t.Fatalf("Expected All to be %t but got %t", c.expectedAll, options.All)
		}

		if c.expectedSize != options.Size {
			t.Fatalf("Expected Size to be %t but got %t", c.expectedSize, options.Size)
		}

		if c.expectedLimit != options.Limit {
			t.Fatalf("Expected Limit to be %d but got %d", c.expectedLimit, options.Limit)
		}

		f := options.Filter

		if f.Len() != len(c.expectedFilters) {
			t.Fatalf("Expected %d filters but got %d", len(c.expectedFilters), f.Len())
		}

		for k, v := range c.expectedFilters {
			f := options.Filter
			if !f.ExactMatch(k, v) {
				t.Fatalf("Expected filter with key %s to be %s but got %s", k, v, f.Get(k))
			}
		}
	}
}
