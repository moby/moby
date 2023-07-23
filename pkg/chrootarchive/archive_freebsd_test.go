package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"

import (
	"testing"

	"github.com/docker/docker/pkg/reexec"
)

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	m.Run()
}
