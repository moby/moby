package daemon

import (
	"os"
	"path/filepath"
	"reflect"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/introspection"
	"github.com/docker/docker/pkg/ioutils"
)

type introspectionOptions struct {
	scopes []string
}

// updateIntrospection updates the actual content of the inspection volume.
//
// The layout is defined as the RuntimeContext structure.
//
func (daemon *Daemon) updateIntrospection(c *container.Container, opts *introspectionOptions) error {
	ctx := daemon.introspectRuntimeContext(c)
	ref := reflect.ValueOf(ctx)
	if err := introspection.VerifyScopes(opts.scopes, ref); err != nil {
		return err
	}
	conn := &fsIntrospectionConnector{dir: c.IntrospectionDir()}
	return introspection.Update(conn, opts.scopes, ref)
}

// fsIntrospectionConnector implements introspection.Connector
type fsIntrospectionConnector struct {
	dir string
}

func (conn *fsIntrospectionConnector) Update(scope, path string, content []byte, perm os.FileMode) error {
	if perm&os.ModeDir != 0 {
		// our connector should not create empty dir.
		// so, mkdir is called only where it is required by regular files
		return nil
	}
	realPath := filepath.Join(conn.dir, path)
	if err := os.MkdirAll(filepath.Dir(realPath), 0755); err != nil {
		return err
	}
	if content == nil {
		return os.RemoveAll(realPath)
	}
	return ioutils.AtomicWriteFile(realPath, content, perm)
}
