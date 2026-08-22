package cluster

import (
	"context"
	"testing"

	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
)

func TestDefaultServiceName(t *testing.T) {
	t.Run("blank name is generated", func(t *testing.T) {
		t.Parallel()

		var retry int
		c := &Cluster{config: Config{GenerateName: func(_ context.Context, gotRetry int) (string, error) {
			retry = gotRetry
			return "generated-name", nil
		}}}
		spec := swarm.ServiceSpec{}

		assert.NilError(t, c.defaultServiceName(context.Background(), &spec))
		assert.Equal(t, spec.Name, "generated-name")
		assert.Equal(t, retry, 0)
	})

	t.Run("explicit name is unchanged", func(t *testing.T) {
		t.Parallel()

		called := false
		c := &Cluster{config: Config{GenerateName: func(context.Context, int) (string, error) {
			called = true
			return "generated-name", nil
		}}}
		spec := swarm.ServiceSpec{Annotations: swarm.Annotations{Name: "explicit-name"}}

		assert.NilError(t, c.defaultServiceName(context.Background(), &spec))
		assert.Equal(t, spec.Name, "explicit-name")
		assert.Assert(t, !called)
	})
}
