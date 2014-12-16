package host

import (
	"os"
	"testing"

	"github.com/docker/docker/pkg/stringutils"
)

// os.Stat(v.Path) is NOT returning ErrNotExist so skip and return error from
// initialize
func TestCannotStatPathFileNameTooLong(t *testing.T) {
	driver := &Driver{}

	err := driver.Create(stringutils.GenerateRandomAlphaOnlyString(300))
	if err == nil {
		t.Fatal("Expected not to mkdir with path that is too long")
	}

	if os.IsNotExist(err) {
		t.Fatal("Expected to not get ErrNotExist")
	}
}
