package daemon // import "github.com/docker/docker/integration/daemon"

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
