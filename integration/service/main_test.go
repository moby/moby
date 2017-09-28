package service

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/internal/test/environment"
	"github.com/gotestyourself/gotestyourself/poll"
)

var testEnv *environment.Execution

const dockerdBinary = "dockerd"

func TestMain(m *testing.M) {
	var err error
	testEnv, err = environment.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	testEnv.Print()
	os.Exit(m.Run())
}

func setupTest(t *testing.T) func() {
	environment.ProtectAll(t, testEnv)
	return func() { testEnv.Clean(t) }
}

// Default poll settings
func pollSettings(config *poll.Settings) {
	config.Delay = 100 * time.Millisecond
	config.Timeout = 20 * time.Second
}
