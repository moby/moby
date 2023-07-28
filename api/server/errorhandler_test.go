package server

import (
	"errors"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/hashicorp/go-multierror"
	"gotest.tools/v3/assert"
)

func TestUnwrapErrors(t *testing.T) {
	testcases := map[string]struct {
		err      error
		expected *types.ErrorResponse
	}{
		"non-HTTP error": {
			err:      errors.New("foobar"),
			expected: &types.ErrorResponse{Message: "foobar"},
		},
		"error wrapped by a HTTP-only error": {
			err:      errdefs.InvalidParameter(errors.New("foobar")),
			expected: &types.ErrorResponse{Message: "foobar"},
		},
		"error wrapped multiple times with a HTTP-only error": {
			err:      errdefs.InvalidParameter(errdefs.Conflict(errors.New("foobar"))),
			expected: &types.ErrorResponse{Message: "foobar"},
		},
		"wrapping error with no context": {
			err:      fmt.Errorf("%w", errors.New("foobar")),
			expected: &types.ErrorResponse{Message: "foobar", Errors: []*types.ErrorResponse{{Message: "foobar"}}},
		},
		"tree with errors.Join": {
			err: errors.Join(
				errors.New("foo"),
				errors.Join(errors.New("bar"), errors.New("baz")),
				errors.New("one more error"),
			),
			expected: &types.ErrorResponse{
				Message: `3 errors occurred:
	* foo
	* 2 errors occurred:
		* bar
		* baz
	* one more error`,
				Errors: []*types.ErrorResponse{
					{Message: "foo"},
					{
						Message: `2 errors occurred:
	* bar
	* baz`,
						Errors: []*types.ErrorResponse{
							{Message: "bar"},
							{Message: "baz"},
						},
					},
					{Message: "one more error"},
				},
			},
		},
		"page not found error": {err: pageNotFoundError{}, expected: &types.ErrorResponse{Message: "page not found"}},
		"multi %w verb": {
			err: fmt.Errorf("foo: %w, %w", errors.New("bar"), errors.New("baz")),
			expected: &types.ErrorResponse{
				Message: "foo: bar, baz",
				Errors: []*types.ErrorResponse{
					{Message: "bar"},
					{Message: "baz"},
				},
			},
		},
		"tree with github.com/hashicorp/go-multierror, multi %w verbs and errors.Join": {
			err: multierror.Append(
				errors.New("foo"),
				fmt.Errorf("bar: %w, %w", errors.New("baz"), errors.New("blah")),
				errors.Join(errors.New("one more error"), errors.New("and a last one"))),
			expected: &types.ErrorResponse{
				Message: `3 errors occurred:
	* foo
	* bar: baz, blah
	* 2 errors occurred:
		* one more error
		* and a last one`,
				Errors: []*types.ErrorResponse{
					{Message: "foo"},
					{
						Message: "bar: baz, blah",
						Errors: []*types.ErrorResponse{
							{Message: "baz"},
							{Message: "blah"},
						},
					},
					{
						Message: `2 errors occurred:
	* one more error
	* and a last one`,
						Errors: []*types.ErrorResponse{
							{Message: "one more error"},
							{Message: "and a last one"},
						},
					},
				},
			},
		},
	}

	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			result := marshalErrorResponse(tc.err)
			assert.DeepEqual(t, tc.expected, result)
		})
	}
}
