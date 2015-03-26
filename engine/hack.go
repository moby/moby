package engine

type Hack map[string]interface{}

func (eng *Engine) HackGetGlobalVar(key string) interface{} {
	if eng.hack == nil {
		return nil
	}
	val, exists := eng.hack[key]
	if !exists {
		return nil
	}
	return val
}

func (eng *Engine) HackSetGlobalVar(key string, val interface{}) {
	if eng.hack == nil {
		eng.hack = make(Hack)
	}
	eng.hack[key] = val
}
