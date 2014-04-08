package dockerfile

import (
	"fmt"
	"strings"
	"reflect"
)

func ReflectorHandler(b interface{}) Handler {
	return reflectorHandler{b}
}

type reflectorHandler struct {
	b interface{}
}

func (r reflectorHandler) Handle(stepname, cmd, arg string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("empty command")
	}
	method, exists := reflect.TypeOf(r.b).MethodByName("Cmd" + strings.ToUpper(cmd[:1]) + strings.ToLower(cmd[1:]))
	if !exists {
		return fmt.Errorf("No such command: %s", cmd)
	}
	ret := method.Func.Call([]reflect.Value{reflect.ValueOf(r.b), reflect.ValueOf(arg)})[0].Interface()
	if ret != nil {
		return ret.(error)
	}
	return nil
}
