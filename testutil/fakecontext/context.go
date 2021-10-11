package fakecontext // import "github.com/docker/docker/testutil/fakecontext"

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/pkg/archive"
)

// New creates a fake build context
func New(t testing.TB, dir string, modifiers ...func(*Fake) error) *Fake {
	t.Helper()
	fakeContext := &Fake{Dir: dir}
	if dir == "" {
		if err := newDir(fakeContext); err != nil {
			t.Fatal(err)
		}
	}

	for _, modifier := range modifiers {
		if err := modifier(fakeContext); err != nil {
			t.Fatal(err)
		}
	}

	return fakeContext
}

func newDir(fake *Fake) error {
	tmp, err := os.MkdirTemp("", "fake-context")
	if err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		return err
	}
	fake.Dir = tmp
	return nil
}

// WithFile adds the specified file (with content) in the build context
func WithFile(name, content string) func(*Fake) error {
	return func(ctx *Fake) error {
		return ctx.Add(name, content)
	}
}

// WithDockerfile adds the specified content as Dockerfile in the build context
func WithDockerfile(content string) func(*Fake) error {
	return WithFile("Dockerfile", content)
}

// WithFiles adds the specified files in the build context, content is a string
func WithFiles(files map[string]string) func(*Fake) error {
	return func(fakeContext *Fake) error {
		for file, content := range files {
			if err := fakeContext.Add(file, content); err != nil {
				return err
			}
		}
		return nil
	}
}

// WithBinaryFiles adds the specified files in the build context, content is binary
func WithBinaryFiles(files map[string]*bytes.Buffer) func(*Fake) error {
	return func(fakeContext *Fake) error {
		for file, content := range files {
			if err := fakeContext.Add(file, content.String()); err != nil {
				return err
			}
		}
		return nil
	}
}

// Fake creates directories that can be used as a build context
type Fake struct {
	Dir string
}

// Add a file at a path, creating directories where necessary
func (f *Fake) Add(file, content string) error {
	return f.addFile(file, []byte(content))
}

func (f *Fake) addFile(file string, content []byte) error {
	fp := filepath.Join(f.Dir, filepath.FromSlash(file))
	dirpath := filepath.Dir(fp)
	if dirpath != "." {
		if err := os.MkdirAll(dirpath, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(fp, content, 0644)

}

// Delete a file at a path
func (f *Fake) Delete(file string) error {
	fp := filepath.Join(f.Dir, filepath.FromSlash(file))
	return os.RemoveAll(fp)
}

// Close deletes the context
func (f *Fake) Close() error {
	return os.RemoveAll(f.Dir)
}

// AsTarReader returns a ReadCloser with the contents of Dir as a tar archive.
func (f *Fake) AsTarReader(t testing.TB) io.ReadCloser {
	t.Helper()
	reader, err := archive.TarWithOptions(f.Dir, &archive.TarOptions{})
	if err != nil {
		t.Fatalf("Failed to create tar from %s: %s", f.Dir, err)
	}
	return reader
}
