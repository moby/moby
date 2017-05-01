package continuity

import "os"

// newBaseResource returns a *resource, populated with data from p and fi,
// where p will be populated directly.
func newBaseResource(p string, fi os.FileInfo) (*resource, error) {
	return &resource{
		paths: []string{p},
		mode:  fi.Mode(),
	}, nil
}
