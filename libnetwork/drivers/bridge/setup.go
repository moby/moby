package bridge

type SetupStep func(*Interface) error

type BridgeSetup struct {
	bridge *Interface
	steps  []SetupStep
}

func NewBridgeSetup(i *Interface) *BridgeSetup {
	return &BridgeSetup{bridge: i}
}

func (b *BridgeSetup) Apply() error {
	for _, fn := range b.steps {
		if err := fn(b.bridge); err != nil {
			return err
		}
	}
	return nil
}

func (b *BridgeSetup) QueueStep(step SetupStep) {
	b.steps = append(b.steps, step)
}

//---------------------------------------------------------------------------//

func SetupIPTables(i *Interface) error {
	return nil
}
