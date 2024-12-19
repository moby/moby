//go:build unix && !linux

package chrootarchive

import (
	"testing"

	"github.com/moby/sys/reexec"
)

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	m.Run()
}
