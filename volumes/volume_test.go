package volumes

import "testing"

func TestContainers(t *testing.T) {
	v := &Volume{containers: make(map[string]struct{})}
	id := "1234"

	v.AddContainer(id)

	if v.Containers()[0] != id {
		t.Fatalf("adding a container ref failed")
	}

	v.RemoveContainer(id)
	if len(v.Containers()) != 0 {
		t.Fatalf("removing container failed")
	}
}
