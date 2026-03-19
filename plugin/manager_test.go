package plugin

import (
	"testing"

	"github.com/docker/docker/api/types"
)

func TestValidatePrivileges(t *testing.T) {
	testData := map[string]struct {
		requiredPrivileges types.PluginPrivileges
		privileges         types.PluginPrivileges
		result             bool
	}{
		"diff-len": {
			requiredPrivileges: []types.PluginPrivilege{
				{Name: "Privilege1", Description: "Description", Value: []string{"abc", "def", "ghi"}},
			},
			privileges: []types.PluginPrivilege{
				{Name: "Privilege1", Description: "Description", Value: []string{"abc", "def", "ghi"}},
				{Name: "Privilege2", Description: "Description", Value: []string{"123", "456", "789"}},
			},
			result: false,
		},
		"diff-value": {
			requiredPrivileges: []types.PluginPrivilege{
				{Name: "Privilege1", Description: "Description", Value: []string{"abc", "def", "GHI"}},
				{Name: "Privilege2", Description: "Description", Value: []string{"123", "456", "***"}},
			},
			privileges: []types.PluginPrivilege{
				{Name: "Privilege1", Description: "Description", Value: []string{"abc", "def", "ghi"}},
				{Name: "Privilege2", Description: "Description", Value: []string{"123", "456", "789"}},
			},
			result: false,
		},
		"diff-order-but-same-value": {
			requiredPrivileges: []types.PluginPrivilege{
				{Name: "Privilege1", Description: "Description", Value: []string{"abc", "def", "GHI"}},
				{Name: "Privilege2", Description: "Description", Value: []string{"123", "456", "789"}},
			},
			privileges: []types.PluginPrivilege{
				{Name: "Privilege2", Description: "Description", Value: []string{"123", "456", "789"}},
				{Name: "Privilege1", Description: "Description", Value: []string{"GHI", "abc", "def"}},
			},
			result: true,
		},
		"single-element-same": {
			requiredPrivileges: []types.PluginPrivilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"true"}},
			},
			privileges: []types.PluginPrivilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"true"}},
			},
			result: true,
		},
		"single-element-diff-value": {
			requiredPrivileges: []types.PluginPrivilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"false"}},
			},
			privileges: []types.PluginPrivilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"true"}},
			},
			result: false,
		},
		"first-sorted-element-diff-value": {
			requiredPrivileges: []types.PluginPrivilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"false"}},
				{Name: "network", Description: "Description", Value: []string{"host"}},
			},
			privileges: []types.PluginPrivilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"true"}},
				{Name: "network", Description: "Description", Value: []string{"host"}},
			},
			result: false,
		},
		"empty-privileges": {
			requiredPrivileges: []types.PluginPrivilege{},
			privileges:         []types.PluginPrivilege{},
			result:             true,
		},
	}

	for key, data := range testData {
		t.Run(key, func(t *testing.T) {
			err := validatePrivileges(data.requiredPrivileges, data.privileges)
			if (err == nil) != data.result {
				t.Fatalf("expected result to be %t, got %t", data.result, (err == nil))
			}
		})
	}
}
