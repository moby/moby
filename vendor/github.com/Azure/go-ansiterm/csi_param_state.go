package ansiterm

import "slices"

type csiParamState struct {
	baseState
}

func (csiState csiParamState) Handle(b byte) (s state, e error) {
	csiState.parser.logf("CsiParam::Handle %#x", b)

	nextState, err := csiState.baseState.Handle(b)
	if nextState != nil || err != nil {
		return nextState, err
	}

	switch {
	case slices.Contains(alphabetics, b):
		return csiState.parser.ground, nil
	case slices.Contains(csiCollectables, b):
		return csiState, csiState.parser.collectParam()
	case slices.Contains(executors, b):
		return csiState, csiState.parser.execute()
	}

	return csiState, nil
}

func (csiState csiParamState) Transition(s state) error {
	csiState.parser.logf("CsiParam::Transition %s --> %s", csiState.Name(), s.Name())
	_ = csiState.baseState.Transition(s)

	switch s {
	case csiState.parser.ground:
		return csiState.parser.csiDispatch()
	}

	return nil
}
