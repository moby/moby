package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"testing"

	"github.com/docker/docker/builder/dockerfile/instructions"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/internal/testutil"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
)

type dispatchTestCase struct {
	name, expectedError string
	cmd                 instructions.Command
	files               map[string]string
}

func init() {
	reexec.Init()
}

func initDispatchTestCases() []dispatchTestCase {
	dispatchTestCases := []dispatchTestCase{
		{
			name: "ADD multiple files to file",
			cmd: &instructions.AddCommand{SourcesAndDest: instructions.SourcesAndDest{
				"file1.txt",
				"file2.txt",
				"test",
			}},
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name: "Wildcard ADD multiple files to file",
			cmd: &instructions.AddCommand{SourcesAndDest: instructions.SourcesAndDest{
				"file*.txt",
				"test",
			}},
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name: "COPY multiple files to file",
			cmd: &instructions.CopyCommand{SourcesAndDest: instructions.SourcesAndDest{
				"file1.txt",
				"file2.txt",
				"test",
			}},
			expectedError: "When using COPY with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name: "ADD multiple files to file with whitespace",
			cmd: &instructions.AddCommand{SourcesAndDest: instructions.SourcesAndDest{
				"test file1.txt",
				"test file2.txt",
				"test",
			}},
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"test file1.txt": "test1", "test file2.txt": "test2"},
		},
		{
			name: "COPY multiple files to file with whitespace",
			cmd: &instructions.CopyCommand{SourcesAndDest: instructions.SourcesAndDest{
				"test file1.txt",
				"test file2.txt",
				"test",
			}},
			expectedError: "When using COPY with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"test file1.txt": "test1", "test file2.txt": "test2"},
		},
		{
			name: "COPY wildcard no files",
			cmd: &instructions.CopyCommand{SourcesAndDest: instructions.SourcesAndDest{
				"file*.txt",
				"/tmp/",
			}},
			expectedError: "COPY failed: no source files were specified",
			files:         nil,
		},
		{
			name: "COPY url",
			cmd: &instructions.CopyCommand{SourcesAndDest: instructions.SourcesAndDest{
				"https://index.docker.io/robots.txt",
				"/",
			}},
			expectedError: "source can't be a URL for COPY",
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

	context, err := remotecontext.FromArchive(tarStream)

	if err != nil {
		t.Fatalf("Error when creating tar context: %s", err)
	}

	defer func() {
		if err = context.Close(); err != nil {
			t.Fatalf("Error when closing tar context: %s", err)
		}
	}()

	b := newBuilderWithMockBackend()
	sb := newDispatchRequest(b, '`', context, newBuildArgs(make(map[string]*string)), newStagesBuildResults())
	err = dispatch(sb, testCase.cmd)
	testutil.ErrorContains(t, err, testCase.expectedError)
}
