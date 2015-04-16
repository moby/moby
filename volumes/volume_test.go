package volumes

import (
	"os"
	"testing"

	"github.com/docker/docker/pkg/stringutils"
)

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

// os.Stat(v.Path) is returning ErrNotExist, initialize catch it and try to
// mkdir v.Path but it dies and correctly returns the error
func TestInitializeCannotMkdirOnNonExistentPath(t *testing.T) {
	v := &Volume{Path: "nonexistentpath"}

	err := v.initialize()
	if err == nil {
		t.Fatal("Expected not to initialize volume with a non existent path")
	}

	if !os.IsNotExist(err) {
		t.Fatalf("Expected to get ErrNotExist error, got %s", err)
	}
}

// os.Stat(v.Path) is NOT returning ErrNotExist so skip and return error from
// initialize
func TestInitializeCannotStatPathFileNameTooLong(t *testing.T) {
	// ENAMETOOLONG
	v := &Volume{Path: stringutils.GenerateRandomAlphaOnlyString(300)}

	err := v.initialize()
	if err == nil {
		t.Fatal("Expected not to initialize volume with a non existent path")
	}

	if os.IsNotExist(err) {
		t.Fatal("Expected to not get ErrNotExist")
	}
}
