package filesync

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/docker/docker/client/session"
	"github.com/docker/docker/client/session/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestFileSyncIncludePatterns(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "fsynctest")
	require.NoError(t, err)

	destDir, err := ioutil.TempDir("", "fsynctest")
	require.NoError(t, err)

	err = ioutil.WriteFile(filepath.Join(tmpDir, "foo"), []byte("content1"), 0600)
	require.NoError(t, err)

	err = ioutil.WriteFile(filepath.Join(tmpDir, "bar"), []byte("content2"), 0600)
	require.NoError(t, err)

	s, err := session.NewSession("foo", "bar")
	require.NoError(t, err)

	m, err := session.NewManager()
	require.NoError(t, err)

	fs := NewFSSyncProvider(tmpDir, nil)
	s.Allow(fs)

	dialer := session.Dialer(testutil.TestStream(testutil.Handler(m.HandleConn)))

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		return s.Run(ctx, dialer)
	})

	g.Go(func() (reterr error) {
		c, err := m.Get(ctx, s.UUID())
		if err != nil {
			return err
		}
		if err := FSSync(ctx, c, FSSendRequestOpt{
			DestDir:         destDir,
			IncludePatterns: []string{"ba*"},
		}); err != nil {
			return err
		}

		_, err = ioutil.ReadFile(filepath.Join(destDir, "foo"))
		assert.Error(t, err)

		dt, err := ioutil.ReadFile(filepath.Join(destDir, "bar"))
		if err != nil {
			return err
		}
		assert.Equal(t, "content2", string(dt))
		return s.Close()
	})

	err = g.Wait()
	require.NoError(t, err)
}
