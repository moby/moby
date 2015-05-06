package bridge

type setupStep func(*NetworkConfiguration, *bridgeInterface) error

type bridgeSetup struct {
	config *NetworkConfiguration
	bridge *bridgeInterface
	steps  []setupStep
}

func newBridgeSetup(c *NetworkConfiguration, i *bridgeInterface) *bridgeSetup {
	return &bridgeSetup{config: c, bridge: i}
}

func (b *bridgeSetup) apply() error {
	for _, fn := range b.steps {
		if err := fn(b.config, b.bridge); err != nil {
			return err
		}
	}
	return nil
}

func (b *bridgeSetup) queueStep(step setupStep) {
	b.steps = append(b.steps, step)
}
