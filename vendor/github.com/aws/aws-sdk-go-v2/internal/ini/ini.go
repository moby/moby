package ini

import (
	"fmt"
	"io"
	"os"
)

// OpenFile takes a path to a given file, and will open  and parse
// that file.
func OpenFile(path string) (sections Sections, err error) {
	f, oerr := os.Open(path)
	if oerr != nil {
		return Sections{}, &UnableToReadFile{Err: oerr}
	}

	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		} else if closeErr != nil {
			err = fmt.Errorf("close error: %v, original error: %w", closeErr, err)
		}
	}()

	return Parse(f, path)
}

// Parse will parse the given file using the shared config
// visitor.
func Parse(f io.Reader, path string) (Sections, error) {
	tree, err := ParseAST(f)
	if err != nil {
		return Sections{}, err
	}

	v := NewDefaultVisitor(path)
	if err = Walk(tree, v); err != nil {
		return Sections{}, err
	}

	return v.Sections, nil
}

// ParseBytes will parse the given bytes and return the parsed sections.
func ParseBytes(b []byte) (Sections, error) {
	tree, err := ParseASTBytes(b)
	if err != nil {
		return Sections{}, err
	}

	v := NewDefaultVisitor("")
	if err = Walk(tree, v); err != nil {
		return Sections{}, err
	}

	return v.Sections, nil
}
