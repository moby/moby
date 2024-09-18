package cluster // import "github.com/docker/docker/daemon/cluster"

import (
	"testing"

	"github.com/docker/docker/api/types/filters"
)

func TestNewListSecretsFilters(t *testing.T) {
	validNameFilter := filters.NewArgs(filters.Arg("name", "test_name"))
	validIDFilter := filters.NewArgs(filters.Arg("id", "7c9009d6720f6de3b492f5"))

	validLabelFilter := filters.NewArgs(
		filters.Arg("label", "type=test"),
		filters.Arg("label", "storage=ssd"),
		filters.Arg("label", "memory"),
	)
	validNamesFilter := filters.NewArgs(filters.Arg("names", "test_name"))

	validAllFilter := filters.NewArgs(
		filters.Arg("name", "nodeName"),
		filters.Arg("id", "7c9009d6720f6de3b492f5"),
		filters.Arg("label", "type=test"),
		filters.Arg("label", "memory"),
		filters.Arg("names", "test_name"),
	)

	validFilters := []filters.Args{
		validNameFilter,
		validIDFilter,
		validLabelFilter,
		validNamesFilter,
		validAllFilter,
	}

	invalidTypeFilter := filters.NewArgs(filters.Arg("nonexist", "aaaa"))

	invalidFilters := []filters.Args{
		invalidTypeFilter,
	}

	for _, filter := range validFilters {
		if _, err := newListSecretsFilters(filter); err != nil {
			t.Fatalf("Should get no error, got %v", err)
		}
	}

	for _, filter := range invalidFilters {
		if _, err := newListSecretsFilters(filter); err == nil {
			t.Fatalf("Should get an error for filter %v, while got nil", filter)
		}
	}
}

func TestNewListConfigsFilters(t *testing.T) {
	validNameFilter := filters.NewArgs(filters.Arg("name", "test_name"))
	validIDFilter := filters.NewArgs(filters.Arg("id", "7c9009d6720f6de3b492f5"))

	validLabelFilter := filters.NewArgs(
		filters.Arg("label", "type=test"),
		filters.Arg("label", "storage=ssd"),
		filters.Arg("label", "memory"),
	)

	validAllFilter := filters.NewArgs(
		filters.Arg("name", "nodeName"),
		filters.Arg("id", "7c9009d6720f6de3b492f5"),
		filters.Arg("label", "type=test"),
		filters.Arg("label", "memory"),
	)

	validFilters := []filters.Args{
		validNameFilter,
		validIDFilter,
		validLabelFilter,
		validAllFilter,
	}

	invalidTypeFilter := filters.NewArgs(filters.Arg("nonexist", "aaaa"))

	invalidFilters := []filters.Args{
		invalidTypeFilter,
	}

	for _, filter := range validFilters {
		if _, err := newListConfigsFilters(filter); err != nil {
			t.Fatalf("Should get no error, got %v", err)
		}
	}

	for _, filter := range invalidFilters {
		if _, err := newListConfigsFilters(filter); err == nil {
			t.Fatalf("Should get an error for filter %v, while got nil", filter)
		}
	}
}
