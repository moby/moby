package testutil

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ErrorContains checks that the error is not nil, and contains the expected
// substring.
func ErrorContains(t require.TestingT, err error, expectedError string, msgAndArgs ...interface{}) {
	require.Error(t, err, msgAndArgs...)
	assert.Contains(t, err.Error(), expectedError, msgAndArgs...)
}
