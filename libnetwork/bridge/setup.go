package bridge

type SetupStep func(*Interface) error

type BridgeSetup struct {
	bridge *Interface
	steps  []SetupStep
}

func NewBridgeSetup(b *Interface) *BridgeSetup {
	return &BridgeSetup{bridge: b}
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

func SetupFixedCIDRv4(b *Interface) error {
	return nil
}

func SetupFixedCIDRv6(b *Interface) error {
	return nil
}

func SetupIPTables(b *Interface) error {
	return nil
}

func SetupIPForwarding(b *Interface) error {
	return nil
}
