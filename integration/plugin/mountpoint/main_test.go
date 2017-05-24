package mountpoint

import (
	"fmt"
	"os"
	"testing"

	"github.com/docker/docker/internal/test/environment"
	"github.com/docker/docker/pkg/sysinfo"
)

var testEnv *environment.Execution
var sysInfo *sysinfo.SysInfo

const dockerdBinary = "dockerd"

func TestMain(m *testing.M) {
	var err error
	testEnv, err = environment.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	sysInfo = sysinfo.New(true)

	testEnv.Print()
	setupSuite()
	exitCode := m.Run()
	teardownSuite()

	os.Exit(exitCode)
}
