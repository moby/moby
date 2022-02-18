package daemon // import "github.com/moby/moby/integration/daemon"

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
