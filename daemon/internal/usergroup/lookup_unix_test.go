//go:build !windows

package usergroup

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestLookupUserAndGroupThatDoesNotExist(t *testing.T) {
	fakeUser := "fakeuser"
	_, err := LookupUser(fakeUser)
	assert.Check(t, is.Error(err, `getent unable to find entry "fakeuser" in passwd database`))

	_, err = LookupUID(-1)
	assert.Check(t, is.ErrorContains(err, ""))

	fakeGroup := "fakegroup"
	_, err = LookupGroup(fakeGroup)
	assert.Check(t, is.Error(err, `getent unable to find entry "fakegroup" in group database`))

	_, err = LookupGID(-1)
	assert.Check(t, is.ErrorContains(err, ""))
}
