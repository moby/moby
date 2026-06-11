package daemon

import (
	"testing"

	"github.com/moby/moby/api/types/container"
)

func TestToContainerdResources_Defaults(t *testing.T) {
	r, err := toContainerdResources(container.Resources{})
	if err != nil {
		t.Fatal(err)
	}
	checkResourcesAreUnset(t, r)
}
