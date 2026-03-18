package dockerversion

import (
	"context"
	"testing"

	"github.com/moby/moby/v2/pkg/useragent"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDockerUserAgent(t *testing.T) {
	t.Run("daemon user-agent", func(t *testing.T) {
		ua := DockerUserAgent(t.Context())
		expected := getDaemonUserAgent()
		assert.Check(t, is.Equal(ua, expected))
	})

	t.Run("daemon user-agent custom metadata", func(t *testing.T) {
		ua := DockerUserAgent(t.Context(), useragent.VersionInfo{Name: "hello", Version: "world"}, useragent.VersionInfo{Name: "foo", Version: "bar"})
		expected := getDaemonUserAgent() + ` hello/world foo/bar`
		assert.Check(t, is.Equal(ua, expected))
	})

	t.Run("daemon user-agent with upstream", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), UAStringKey{}, "Magic-Client/1.2.3 (linux)")
		ua := DockerUserAgent(ctx)
		expected := getDaemonUserAgent() + ` UpstreamClient(Magic-Client/1.2.3 \(linux\))`
		assert.Check(t, is.Equal(ua, expected))
	})

	t.Run("daemon user-agent with upstream and custom metadata", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), UAStringKey{}, "Magic-Client/1.2.3 (linux)")
		ua := DockerUserAgent(ctx, useragent.VersionInfo{Name: "hello", Version: "world"}, useragent.VersionInfo{Name: "foo", Version: "bar"})
		expected := getDaemonUserAgent() + ` hello/world foo/bar UpstreamClient(Magic-Client/1.2.3 \(linux\))`
		assert.Check(t, is.Equal(ua, expected))
	})
}
