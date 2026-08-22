package client

import (
	"context"
	"errors"
	"testing"

	"github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
)

func TestChainPrivilegeFuncs(t *testing.T) {
	tests := []struct {
		doc            string
		privilegeFunc  registry.RequestAuthConfig
		expected       []string
		checkExhausted bool
	}{
		{
			doc:      "no privilege func",
			expected: []string{""},
		},
		{
			doc:           "no a chain",
			privilegeFunc: staticAuth("A"),
			expected:      []string{"A"},
		},
		{
			doc: "not a chain with error",
			privilegeFunc: func(ctx context.Context) (string, error) {
				return "", errors.New("terminal error: failed to request auth")
			},
		},
		{
			doc:            "empty chain",
			privilegeFunc:  ChainPrivilegeFuncs(),
			checkExhausted: true,
		},
		{
			doc:            "chain with only nil values",
			privilegeFunc:  ChainPrivilegeFuncs(nil, nil),
			checkExhausted: true,
		},
		{
			doc:            "single chain",
			privilegeFunc:  ChainPrivilegeFuncs(staticAuth("A")),
			expected:       []string{"A"},
			checkExhausted: true,
		},
		{
			doc:           "basic chain",
			privilegeFunc: ChainPrivilegeFuncs(staticAuth("A"), staticAuth("B")),
			expected:      []string{"A", "B"},
		},
		{
			doc:            "basic chain with nil values",
			privilegeFunc:  ChainPrivilegeFuncs(staticAuth("A"), nil, staticAuth("B")),
			expected:       []string{"A", "B"},
			checkExhausted: true,
		},
		{
			doc: "chain with nested chains",
			privilegeFunc: ChainPrivilegeFuncs(
				staticAuth("A"),
				staticAuth("B"),
				ChainPrivilegeFuncs(
					staticAuth("C-1"),
					staticAuth("C-2"),
					ChainPrivilegeFuncs(
						staticAuth("C-2-A"),
						staticAuth("C-2-B"),
					),
					staticAuth("C-3"),
				),
				staticAuth("D"),
				func(ctx context.Context) (string, error) {
					return "", errors.New("terminal error: failed to request auth")
				},
				// Should not be used due to terminal error above.
				staticAuth("E"),
			),
			expected:       []string{"A", "B", "C-1", "C-2", "C-2-A", "C-2-B", "C-3", "D"},
			checkExhausted: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			assertAuthChain(t, tc.privilegeFunc, tc.expected)
			if tc.checkExhausted {
				ac, err := tc.privilegeFunc(context.Background())
				assert.Equal(t, ac, "", "should not have returned auth after being exhausted")
				assert.ErrorIs(t, err, errNoMorePrivilegeFuncs, "should be exhausted")
			}
		})
	}
}

func assertAuthChain(t *testing.T, privilegeFunc registry.RequestAuthConfig, expectedAuth []string) {
	t.Helper()
	var actual []string

	var i int
	for {
		ac, shouldRetry, err := getAuth(context.Background(), privilegeFunc)
		if err != nil {
			// t.Logf("terminating after: %v", err)
			break
		}
		actual = append(actual, ac)
		// t.Logf("iteration %d: %s", i, strings.Join(actual, ", "))
		if !shouldRetry {
			break
		}

		// Safety-net in tests.
		i++
		if i > 10 {
			t.Fatal("too many calls to chain")
		}
	}

	assert.DeepEqual(t, actual, expectedAuth)
}
