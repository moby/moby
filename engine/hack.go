package engine

type Hack map[string]interface{}

func (eng *Engine) Hack_GetGlobalVar(key string) interface{} {
	eng.RLock()
	defer eng.RUnlock()

	if eng.hack == nil {
		return nil
	}
	val, exists := eng.hack[key]
	if !exists {
		return nil
	}
	return val
}

func (eng *Engine) Hack_SetGlobalVar(key string, val interface{}) {
	eng.Lock()
	defer eng.Unlock()

	if eng.hack == nil {
		eng.hack = make(Hack)
	}
	eng.hack[key] = val
}
