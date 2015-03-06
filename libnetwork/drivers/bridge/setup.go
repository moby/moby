package bridge

type setupStep func(*bridgeInterface) error

type bridgeSetup struct {
	bridge *bridgeInterface
	steps  []setupStep
}

func newBridgeSetup(i *bridgeInterface) *bridgeSetup {
	return &bridgeSetup{bridge: i}
}

func (b *bridgeSetup) apply() error {
	for _, fn := range b.steps {
		if err := fn(b.bridge); err != nil {
			return err
		}
	}
	return nil
}

func (b *bridgeSetup) queueStep(step setupStep) {
	b.steps = append(b.steps, step)
}

//---------------------------------------------------------------------------//

func setupIPTables(i *bridgeInterface) error {
	return nil
}
