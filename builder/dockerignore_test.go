package builder

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessShouldRemoveDockerfileDockerignore(t *testing.T) {
	contextDir, err := ioutil.TempDir("", "builder-dockerignore-process-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	defer os.RemoveAll(contextDir)

	testFilename := filepath.Join(contextDir, "should_stay")
	testContent := "test"
	err = ioutil.WriteFile(testFilename, []byte(testContent), 0777)

	if err != nil {
		t.Fatalf("Error when creating should_stay file: %s", err)
	}

	dockerignoreFilename := filepath.Join(contextDir, ".dockerignore")
	dockerignoreContent := "Dockerfile\n.dockerignore"
	err = ioutil.WriteFile(dockerignoreFilename, []byte(dockerignoreContent), 0777)

	if err != nil {
		t.Fatalf("Error when creating .dockerignore file: %s", err)
	}

	dockerfileFilename := filepath.Join(contextDir, DefaultDockerfileName)
	dockerfileContent := "FROM busybox"
	err = ioutil.WriteFile(dockerfileFilename, []byte(dockerfileContent), 0777)

	if err != nil {
		t.Fatalf("Error when creating Dockerfile file: %s", err)
	}

	modifiableCtx := &tarSumContext{root: contextDir}
	ctx := DockerIgnoreContext{ModifiableContext: modifiableCtx}

	err = ctx.Process([]string{DefaultDockerfileName})

	if err != nil {
		t.Fatalf("Error when executing Process: %s", err)
	}

	files, err := ioutil.ReadDir(contextDir)

	if err != nil {
		t.Fatalf("Could not read directory: %s", err)
	}

	if len(files) != 1 {
		log.Fatal("Directory should contain exactly one file")
	}

	for _, file := range files {
		if "should_stay" != file.Name() {
			log.Fatalf("File %s should not be in the directory", file.Name())
		}
	}

}

func TestProcessNoDockerignore(t *testing.T) {
	contextDir, err := ioutil.TempDir("", "builder-dockerignore-process-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	defer os.RemoveAll(contextDir)

	testFilename := filepath.Join(contextDir, "should_stay")
	testContent := "test"
	err = ioutil.WriteFile(testFilename, []byte(testContent), 0777)

	if err != nil {
		t.Fatalf("Error when creating should_stay file: %s", err)
	}

	dockerfileFilename := filepath.Join(contextDir, DefaultDockerfileName)
	dockerfileContent := "FROM busybox"
	err = ioutil.WriteFile(dockerfileFilename, []byte(dockerfileContent), 0777)

	if err != nil {
		t.Fatalf("Error when creating Dockerfile file: %s", err)
	}

	modifiableCtx := &tarSumContext{root: contextDir}
	ctx := DockerIgnoreContext{ModifiableContext: modifiableCtx}

	ctx.Process([]string{DefaultDockerfileName})

	files, err := ioutil.ReadDir(contextDir)

	if err != nil {
		t.Fatalf("Could not read directory: %s", err)
	}

	if len(files) != 2 {
		log.Fatal("Directory should contain exactly two files")
	}

	for _, file := range files {
		if "should_stay" != file.Name() && DefaultDockerfileName != file.Name() {
			log.Fatalf("File %s should not be in the directory", file.Name())
		}
	}

}

func TestProcessShouldLeaveAllFiles(t *testing.T) {
	contextDir, err := ioutil.TempDir("", "builder-dockerignore-process-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	defer os.RemoveAll(contextDir)

	testFilename := filepath.Join(contextDir, "should_stay")
	testContent := "test"
	err = ioutil.WriteFile(testFilename, []byte(testContent), 0777)

	if err != nil {
		t.Fatalf("Error when creating should_stay file: %s", err)
	}

	dockerignoreFilename := filepath.Join(contextDir, ".dockerignore")
	dockerignoreContent := "input1\ninput2"
	err = ioutil.WriteFile(dockerignoreFilename, []byte(dockerignoreContent), 0777)

	if err != nil {
		t.Fatalf("Error when creating .dockerignore file: %s", err)
	}

	dockerfileFilename := filepath.Join(contextDir, DefaultDockerfileName)
	dockerfileContent := "FROM busybox"
	err = ioutil.WriteFile(dockerfileFilename, []byte(dockerfileContent), 0777)

	if err != nil {
		t.Fatalf("Error when creating Dockerfile file: %s", err)
	}

	modifiableCtx := &tarSumContext{root: contextDir}
	ctx := DockerIgnoreContext{ModifiableContext: modifiableCtx}

	err = ctx.Process([]string{DefaultDockerfileName})

	if err != nil {
		t.Fatalf("Error when executing Process: %s", err)
	}

	files, err := ioutil.ReadDir(contextDir)

	if err != nil {
		t.Fatalf("Could not read directory: %s", err)
	}

	if len(files) != 3 {
		log.Fatal("Directory should contain exactly three files")
	}

	for _, file := range files {
		if "should_stay" != file.Name() && DefaultDockerfileName != file.Name() && ".dockerignore" != file.Name() {
			log.Fatalf("File %s should not be in the directory", file.Name())
		}
	}

}
