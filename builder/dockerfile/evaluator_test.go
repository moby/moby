package dockerfile

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
)

type dispatchTestCase struct {
	name, dockerfile, expectedError string
	files                           map[string]string
}

func init() {
	reexec.Init()
}

func initDispatchTestCases() []dispatchTestCase {
	dispatchTestCases := []dispatchTestCase{{
		name: "copyEmptyWhitespace",
		dockerfile: `COPY
	quux \
      bar`,
		expectedError: "COPY requires at least one argument",
	},
		{
			name:          "ONBUILD forbidden FROM",
			dockerfile:    "ONBUILD FROM scratch",
			expectedError: "FROM isn't allowed as an ONBUILD trigger",
			files:         nil,
		},
		{
			name:          "ONBUILD forbidden MAINTAINER",
			dockerfile:    "ONBUILD MAINTAINER docker.io",
			expectedError: "MAINTAINER isn't allowed as an ONBUILD trigger",
			files:         nil,
		},
		{
			name:          "ARG two arguments",
			dockerfile:    "ARG foo bar",
			expectedError: "ARG requires exactly one argument definition",
			files:         nil,
		},
		{
			name:          "MAINTAINER unknown flag",
			dockerfile:    "MAINTAINER --boo joe@example.com",
			expectedError: "Unknown flag: boo",
			files:         nil,
		},
		{
			name:          "ADD multiple files to file",
			dockerfile:    "ADD file1.txt file2.txt test",
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name:          "JSON ADD multiple files to file",
			dockerfile:    `ADD ["file1.txt", "file2.txt", "test"]`,
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name:          "Wildcard ADD multiple files to file",
			dockerfile:    "ADD file*.txt test",
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name:          "Wildcard JSON ADD multiple files to file",
			dockerfile:    `ADD ["file*.txt", "test"]`,
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name:          "COPY multiple files to file",
			dockerfile:    "COPY file1.txt file2.txt test",
			expectedError: "When using COPY with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name:          "JSON COPY multiple files to file",
			dockerfile:    `COPY ["file1.txt", "file2.txt", "test"]`,
			expectedError: "When using COPY with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name:          "ADD multiple files to file with whitespace",
			dockerfile:    `ADD [ "test file1.txt", "test file2.txt", "test" ]`,
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"test file1.txt": "test1", "test file2.txt": "test2"},
		},
		{
			name:          "COPY multiple files to file with whitespace",
			dockerfile:    `COPY [ "test file1.txt", "test file2.txt", "test" ]`,
			expectedError: "When using COPY with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"test file1.txt": "test1", "test file2.txt": "test2"},
		},
		{
			name:          "COPY wildcard no files",
			dockerfile:    `COPY file*.txt /tmp/`,
			expectedError: "No source files were specified",
			files:         nil,
		},
		{
			name:          "COPY url",
			dockerfile:    `COPY https://index.docker.io/robots.txt /`,
			expectedError: "Source can't be a URL for COPY",
			files:         nil,
		},
		{
			name:          "Chaining ONBUILD",
			dockerfile:    `ONBUILD ONBUILD RUN touch foobar`,
			expectedError: "Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed",
			files:         nil,
		},
		{
			name:          "Invalid instruction",
			dockerfile:    `foo bar`,
			expectedError: "Unknown instruction: FOO",
			files:         nil,
		}}

	return dispatchTestCases
}

func TestDispatch(t *testing.T) {
	testCases := initDispatchTestCases()

	for _, testCase := range testCases {
		executeTestCase(t, testCase)
	}
}

func executeTestCase(t *testing.T, testCase dispatchTestCase) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-dockerfile-test")
	defer cleanup()

	for filename, content := range testCase.files {
		createTestTempFile(t, contextDir, filename, content, 0777)
	}

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

	r := strings.NewReader(testCase.dockerfile)
	n, err := parser.Parse(r)

	if err != nil {
		t.Fatalf("Error when parsing Dockerfile: %s", err)
	}

	config := &container.Config{}
	options := &types.ImageBuildOptions{}

	b := &Builder{runConfig: config, options: options, Stdout: ioutil.Discard, context: context}

	err = b.dispatch(0, n.Children[0])

	if err == nil {
		t.Fatalf("No error when executing test %s", testCase.name)
	}

	if !strings.Contains(err.Error(), testCase.expectedError) {
		t.Fatalf("Wrong error message. Should be \"%s\". Got \"%s\"", testCase.expectedError, err.Error())
	}

}

// createTestTempDir creates a temporary directory for testing.
// It returns the created path and a cleanup function which is meant to be used as deferred call.
// When an error occurs, it terminates the test.
func createTestTempDir(t *testing.T, dir, prefix string) (string, func()) {
	path, err := ioutil.TempDir(dir, prefix)

	if err != nil {
		t.Fatalf("Error when creating directory %s with prefix %s: %s", dir, prefix, err)
	}

	return path, func() {
		err = os.RemoveAll(path)

		if err != nil {
			t.Fatalf("Error when removing directory %s: %s", path, err)
		}
	}
}

// createTestTempFile creates a temporary file within dir with specific contents and permissions.
// When an error occurs, it terminates the test
func createTestTempFile(t *testing.T, dir, filename, contents string, perm os.FileMode) string {
	filePath := filepath.Join(dir, filename)
	err := ioutil.WriteFile(filePath, []byte(contents), perm)

	if err != nil {
		t.Fatalf("Error when creating %s file: %s", filename, err)
	}

	return filePath
}
