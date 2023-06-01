package dockerversion

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDockerUserAgent(t *testing.T) {
	t.Run("daemon user-agent", func(t *testing.T) {
		ua := DockerUserAgent(context.TODO())
		expected := getDaemonUserAgent()
		assert.Check(t, is.Equal(ua, expected))
	})

	t.Run("daemon user-agent with upstream", func(t *testing.T) {
		ctx := context.WithValue(context.TODO(), UAStringKey{}, "Magic-Client/1.2.3 (linux)")
		ua := DockerUserAgent(ctx)
		expected := getDaemonUserAgent() + ` UpstreamClient(Magic-Client/1.2.3 \(linux\))`
		assert.Check(t, is.Equal(ua, expected))
	})
}
