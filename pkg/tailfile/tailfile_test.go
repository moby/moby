package tailfile

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestTailFile(c *check.C) {
	f, err := ioutil.TempFile("", "tail-test")
	if err != nil {
		c.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(f.Name())
	testFile := []byte(`first line
second line
third line
fourth line
fifth line
next first line
next second line
next third line
next fourth line
next fifth line
last first line
next first line
next second line
next third line
next fourth line
next fifth line
next first line
next second line
next third line
next fourth line
next fifth line
last second line
last third line
last fourth line
last fifth line
truncated line`)
	if _, err := f.Write(testFile); err != nil {
		c.Fatal(err)
	}
	if _, err := f.Seek(0, os.SEEK_SET); err != nil {
		c.Fatal(err)
	}
	expected := []string{"last fourth line", "last fifth line"}
	res, err := TailFile(f, 2)
	if err != nil {
		c.Fatal(err)
	}
	for i, l := range res {
		c.Logf("%s", l)
		if expected[i] != string(l) {
			c.Fatalf("Expected line %s, got %s", expected[i], l)
		}
	}
}

func (s *DockerSuite) TestTailFileManyLines(c *check.C) {
	f, err := ioutil.TempFile("", "tail-test")
	if err != nil {
		c.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(f.Name())
	testFile := []byte(`first line
second line
truncated line`)
	if _, err := f.Write(testFile); err != nil {
		c.Fatal(err)
	}
	if _, err := f.Seek(0, os.SEEK_SET); err != nil {
		c.Fatal(err)
	}
	expected := []string{"first line", "second line"}
	res, err := TailFile(f, 10000)
	if err != nil {
		c.Fatal(err)
	}
	for i, l := range res {
		c.Logf("%s", l)
		if expected[i] != string(l) {
			c.Fatalf("Expected line %s, got %s", expected[i], l)
		}
	}
}

func (s *DockerSuite) TestTailEmptyFile(c *check.C) {
	f, err := ioutil.TempFile("", "tail-test")
	if err != nil {
		c.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(f.Name())
	res, err := TailFile(f, 10000)
	if err != nil {
		c.Fatal(err)
	}
	if len(res) != 0 {
		c.Fatal("Must be empty slice from empty file")
	}
}

func (s *DockerSuite) TestTailNegativeN(c *check.C) {
	f, err := ioutil.TempFile("", "tail-test")
	if err != nil {
		c.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(f.Name())
	testFile := []byte(`first line
second line
truncated line`)
	if _, err := f.Write(testFile); err != nil {
		c.Fatal(err)
	}
	if _, err := f.Seek(0, os.SEEK_SET); err != nil {
		c.Fatal(err)
	}
	if _, err := TailFile(f, -1); err != ErrNonPositiveLinesNumber {
		c.Fatalf("Expected ErrNonPositiveLinesNumber, got %s", err)
	}
	if _, err := TailFile(f, 0); err != ErrNonPositiveLinesNumber {
		c.Fatalf("Expected ErrNonPositiveLinesNumber, got %s", err)
	}
}

func (s *DockerSuite) BenchmarkTail(c *check.C) {
	f, err := ioutil.TempFile("", "tail-test")
	if err != nil {
		c.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(f.Name())
	for i := 0; i < 10000; i++ {
		if _, err := f.Write([]byte("tailfile pretty interesting line\n")); err != nil {
			c.Fatal(err)
		}
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		if _, err := TailFile(f, 1000); err != nil {
			c.Fatal(err)
		}
	}
}
