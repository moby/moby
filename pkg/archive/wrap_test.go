package archive

import (
	"archive/tar"
	"bytes"
	"io"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestGenerateEmptyFile(c *check.C) {
	archive, err := Generate("emptyFile")
	if err != nil {
		c.Fatal(err)
	}
	if archive == nil {
		c.Fatal("The generated archive should not be nil.")
	}

	expectedFiles := [][]string{
		{"emptyFile", ""},
	}

	tr := tar.NewReader(archive)
	actualFiles := make([][]string, 0, 10)
	i := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.Fatal(err)
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(tr)
		content := buf.String()
		actualFiles = append(actualFiles, []string{hdr.Name, content})
		i++
	}
	if len(actualFiles) != len(expectedFiles) {
		c.Fatalf("Number of expected file %d, got %d.", len(expectedFiles), len(actualFiles))
	}
	for i := 0; i < len(expectedFiles); i++ {
		actual := actualFiles[i]
		expected := expectedFiles[i]
		if actual[0] != expected[0] {
			c.Fatalf("Expected name '%s', Actual name '%s'", expected[0], actual[0])
		}
		if actual[1] != expected[1] {
			c.Fatalf("Expected content '%s', Actual content '%s'", expected[1], actual[1])
		}
	}
}

func (s *DockerSuite) TestGenerateWithContent(c *check.C) {
	archive, err := Generate("file", "content")
	if err != nil {
		c.Fatal(err)
	}
	if archive == nil {
		c.Fatal("The generated archive should not be nil.")
	}

	expectedFiles := [][]string{
		{"file", "content"},
	}

	tr := tar.NewReader(archive)
	actualFiles := make([][]string, 0, 10)
	i := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.Fatal(err)
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(tr)
		content := buf.String()
		actualFiles = append(actualFiles, []string{hdr.Name, content})
		i++
	}
	if len(actualFiles) != len(expectedFiles) {
		c.Fatalf("Number of expected file %d, got %d.", len(expectedFiles), len(actualFiles))
	}
	for i := 0; i < len(expectedFiles); i++ {
		actual := actualFiles[i]
		expected := expectedFiles[i]
		if actual[0] != expected[0] {
			c.Fatalf("Expected name '%s', Actual name '%s'", expected[0], actual[0])
		}
		if actual[1] != expected[1] {
			c.Fatalf("Expected content '%s', Actual content '%s'", expected[1], actual[1])
		}
	}
}
