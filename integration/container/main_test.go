package container

import (
	"fmt"
	"os"
	"testing"

	"github.com/docker/docker/integration-cli/environment"
)

var (
	testEnv *environment.Execution
)

func TestMain(m *testing.M) {
	var err error
	testEnv, err = environment.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// TODO: replace this with `testEnv.Print()` to print the full env
	if testEnv.LocalDaemon() {
		fmt.Println("INFO: Testing against a local daemon")
	} else {
		fmt.Println("INFO: Testing against a remote daemon")
	}

	res := m.Run()
	os.Exit(res)
}

func setupTest(t *testing.T) func() {
	environment.ProtectImages(t, testEnv)
	return func() { testEnv.Clean(t, testEnv.DockerBinary()) }
}
