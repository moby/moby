package dockerfile

import (
	"strings"
	"testing"

	"github.com/docker/docker/builder"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/engine-api/types"
)

func TestEmptyDockerfile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-dockerfile-test")
	defer cleanup()

	createTestTempFile(t, contextDir, builder.DefaultDockerfileName, "", 0777)

	tarStream, err := archive.Tar(contextDir, archive.Uncompressed)

	if err != nil {
		t.Fatalf("Error when creating tar stream: %s", err)
	}

	defer func() {
		if err = tarStream.Close(); err != nil {
			t.Fatalf("Error when closing tar stream: %s", err)
		}
	}()

	context, err := builder.MakeTarSumContext(tarStream)

	if err != nil {
		t.Fatalf("Error when creating tar context: %s", err)
	}

	defer func() {
		if err = context.Close(); err != nil {
			t.Fatalf("Error when closing tar context: %s", err)
		}
	}()

	options := &types.ImageBuildOptions{}

	b := &Builder{options: options, context: context}

	err = b.readDockerfile()

	if err == nil {
		t.Fatalf("No error when executing test for empty Dockerfile")
	}

	if !strings.Contains(err.Error(), "The Dockerfile (Dockerfile) cannot be empty") {
		t.Fatalf("Wrong error message. Should be \"%s\". Got \"%s\"", "The Dockerfile (Dockerfile) cannot be empty", err.Error())
	}
}
