package networking

import (
	"context"
	"os"
	"testing"

	"github.com/docker/docker/testutil/environment"
)

var testEnv *environment.Execution

func TestMain(m *testing.M) {
	var err error
	testEnv, err = environment.New()
	if err != nil {
		panic(err)
	}

	err = environment.EnsureFrozenImagesLinux(testEnv)
	if err != nil {
		panic(err)
	}

	testEnv.Print()
	code := m.Run()
	os.Exit(code)
}

func setupTest(t *testing.T) context.Context {
	environment.ProtectAll(t, testEnv)
	t.Cleanup(func() { testEnv.Clean(t) })
	return context.Background()
}
