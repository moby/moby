package dockerversion

import (
	"context"
	"testing"

	"github.com/docker/docker/pkg/useragent"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDockerUserAgent(t *testing.T) {
	t.Run("daemon user-agent", func(t *testing.T) {
		ua := DockerUserAgent(context.TODO())
		expected := getDaemonUserAgent()
		assert.Check(t, is.Equal(ua, expected))
	})

	t.Run("daemon user-agent custom metadata", func(t *testing.T) {
		ua := DockerUserAgent(context.TODO(), useragent.VersionInfo{Name: "hello", Version: "world"}, useragent.VersionInfo{Name: "foo", Version: "bar"})
		expected := getDaemonUserAgent() + ` hello/world foo/bar`
		assert.Check(t, is.Equal(ua, expected))
	})

	t.Run("daemon user-agent with upstream", func(t *testing.T) {
		ctx := context.WithValue(context.TODO(), UAStringKey{}, "Magic-Client/1.2.3 (linux)")
		ua := DockerUserAgent(ctx)
		expected := getDaemonUserAgent() + ` UpstreamClient(Magic-Client/1.2.3 \(linux\))`
		assert.Check(t, is.Equal(ua, expected))
	})

	t.Run("daemon user-agent with upstream and custom metadata", func(t *testing.T) {
		ctx := context.WithValue(context.TODO(), UAStringKey{}, "Magic-Client/1.2.3 (linux)")
		ua := DockerUserAgent(ctx, useragent.VersionInfo{Name: "hello", Version: "world"}, useragent.VersionInfo{Name: "foo", Version: "bar"})
		expected := getDaemonUserAgent() + ` hello/world foo/bar UpstreamClient(Magic-Client/1.2.3 \(linux\))`
		assert.Check(t, is.Equal(ua, expected))
	})
}
