//go:build linux

package bridge

import (
	"context"

	"github.com/moby/moby/v2/daemon/internal/otelutil"
	"go.opentelemetry.io/otel"
)

type setupStep struct {
	name string
	fn   stepFn
}

type stepFn func(*networkConfiguration, *bridgeInterface) error

type bridgeSetup struct {
	config *networkConfiguration
	bridge *bridgeInterface
	steps  []setupStep
}

func newBridgeSetup(c *networkConfiguration, i *bridgeInterface) *bridgeSetup {
	return &bridgeSetup{config: c, bridge: i}
}

func (b *bridgeSetup) apply(ctx context.Context) error {
	for _, step := range b.steps {
		ctx, span := otel.Tracer("").Start(ctx, spanPrefix+"."+step.name)
		_ = ctx // To avoid unused variable error while making sure that if / when setupStep starts taking a context, the right value will be used.

		err := step.fn(b.config, b.bridge)
		otelutil.RecordStatus(span, err)
		span.End()

		if err != nil {
			return err
		}
	}
	return nil
}

func (b *bridgeSetup) queueStep(name string, fn stepFn) {
	b.steps = append(b.steps, setupStep{name, fn})
}
