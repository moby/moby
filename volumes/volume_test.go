package volumes

import "testing"

type dummyDriver struct{}

func (d *dummyDriver) Create(path string) error {
	return nil
}
func (d *dummyDriver) Remove(path string) error {
	return nil
}
func (d *dummyDriver) Link(path, id string) error {
	return nil
}
func (d *dummyDriver) Unlink(path, id string) error {
	return nil
}
func (d *dummyDriver) Exists(path string) bool {
	return true
}

func TestContainers(t *testing.T) {
	v := &Volume{containers: make(map[string]struct{}), driver: &dummyDriver{}}
	id := "1234"

	v.Link(id)

	if v.Containers()[0] != id {
		t.Fatalf("adding a container ref failed")
	}

	v.Unlink(id)
	if len(v.Containers()) != 0 {
		t.Fatalf("removing container failed")
	}
}
