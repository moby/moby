package safepath

import (
	"context"
	"fmt"
	"sync"

	"github.com/containerd/log"
)

type SafePath struct {
	path    string
	cleanup func(ctx context.Context) error
	mutex   sync.Mutex

	// Immutable fields
	sourceBase, sourceSubpath string
}

// Close releases the resources used by the path.
func (s *SafePath) Close(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.path == "" {
		base, sub := s.SourcePath()
		log.G(ctx).WithFields(log.Fields{
			"path":          s.Path(),
			"sourceBase":    base,
			"sourceSubpath": sub,
		}).Warn("an attempt to close an already closed SafePath")
		return nil
	}

	s.path = ""
	if s.cleanup != nil {
		return s.cleanup(ctx)
	}
	return nil
}

// IsValid return true when path can still be used and wasn't cleaned up by Close.
func (s *SafePath) IsValid() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.path != ""
}

// Path returns a safe, temporary path that can be used to access the original path.
func (s *SafePath) Path() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.path == "" {
		panic(fmt.Sprintf("use-after-close attempted for safepath with source [%s, %s]", s.sourceBase, s.sourceSubpath))
	}
	return s.path
}

// SourcePath returns the source path the safepath points to.
func (s *SafePath) SourcePath() (string, string) {
	// No mutex lock because these are immutable.
	return s.sourceBase, s.sourceSubpath
}
