package plugin

import (
	"testing"

	"github.com/moby/moby/api/types/plugin"
)

func TestValidatePrivileges(t *testing.T) {
	testData := map[string]struct {
		requiredPrivileges plugin.Privileges
		privileges         plugin.Privileges
		result             bool
	}{
		"diff-len": {
			requiredPrivileges: []plugin.Privilege{
				{Name: "Privilege1", Description: "Description", Value: []string{"abc", "def", "ghi"}},
			},
			privileges: []plugin.Privilege{
				{Name: "Privilege1", Description: "Description", Value: []string{"abc", "def", "ghi"}},
				{Name: "Privilege2", Description: "Description", Value: []string{"123", "456", "789"}},
			},
			result: false,
		},
		"diff-value": {
			requiredPrivileges: []plugin.Privilege{
				{Name: "Privilege1", Description: "Description", Value: []string{"abc", "def", "GHI"}},
				{Name: "Privilege2", Description: "Description", Value: []string{"123", "456", "***"}},
			},
			privileges: []plugin.Privilege{
				{Name: "Privilege1", Description: "Description", Value: []string{"abc", "def", "ghi"}},
				{Name: "Privilege2", Description: "Description", Value: []string{"123", "456", "789"}},
			},
			result: false,
		},
		"diff-order-but-same-value": {
			requiredPrivileges: []plugin.Privilege{
				{Name: "Privilege1", Description: "Description", Value: []string{"abc", "def", "GHI"}},
				{Name: "Privilege2", Description: "Description", Value: []string{"123", "456", "789"}},
			},
			privileges: []plugin.Privilege{
				{Name: "Privilege2", Description: "Description", Value: []string{"123", "456", "789"}},
				{Name: "Privilege1", Description: "Description", Value: []string{"GHI", "abc", "def"}},
			},
			result: true,
		},
		"single-element-same": {
			requiredPrivileges: []plugin.Privilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"true"}},
			},
			privileges: []plugin.Privilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"true"}},
			},
			result: true,
		},
		"single-element-diff-value": {
			requiredPrivileges: []plugin.Privilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"false"}},
			},
			privileges: []plugin.Privilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"true"}},
			},
			result: false,
		},
		"first-sorted-element-diff-value": {
			requiredPrivileges: []plugin.Privilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"false"}},
				{Name: "network", Description: "Description", Value: []string{"host"}},
			},
			privileges: []plugin.Privilege{
				{Name: "allow-all-devices", Description: "Description", Value: []string{"true"}},
				{Name: "network", Description: "Description", Value: []string{"host"}},
			},
			result: false,
		},
		"empty-privileges": {
			requiredPrivileges: []plugin.Privilege{},
			privileges:         []plugin.Privilege{},
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
