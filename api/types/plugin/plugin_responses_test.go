package plugin_test

import (
	"slices"
	"sort"
	"testing"

	"github.com/moby/moby/api/types/plugin"
)

func assertPrivilegesEqual(t *testing.T, got, want plugin.Privileges) {
	t.Helper()

	if !slices.EqualFunc(got, want, func(a, b plugin.Privilege) bool {
		return a.Name == b.Name && slices.Equal(a.Value, b.Value)
	}) {
		t.Fatalf("unexpected privileges:\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestPrivilegesSort(t *testing.T) {
	privileges := plugin.Privileges{
		{Name: "write", Value: []string{"z", "a"}},
		{Name: "read", Value: []string{"y", "b"}},
	}

	sort.Sort(privileges)

	assertPrivilegesEqual(t, privileges, plugin.Privileges{
		{Name: "read", Value: []string{"y", "b"}},
		{Name: "write", Value: []string{"z", "a"}},
	})
}

func TestPrivilegesSwap(t *testing.T) {
	privileges := plugin.Privileges{
		{Name: "foo", Value: []string{"z", "a"}},
		{Name: "bar", Value: []string{"y", "b"}},
	}

	privileges.Swap(0, 1)

	assertPrivilegesEqual(t, privileges, plugin.Privileges{
		{Name: "bar", Value: []string{"y", "b"}},
		{Name: "foo", Value: []string{"z", "a"}},
	})
}
