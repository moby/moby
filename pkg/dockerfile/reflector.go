package dockerfile

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

func ReflectorHandler(b interface{}, stderr io.Writer) Handler {
	return reflectorHandler{b, stderr}
}

type reflectorHandler struct {
	b      interface{}
	stderr io.Writer
}

func (r reflectorHandler) Handle(stepname, cmd, arg string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("empty command")
	}
	method, exists := reflect.TypeOf(r.b).MethodByName("Cmd" + strings.ToUpper(cmd[:1]) + strings.ToLower(cmd[1:]))
	// Gracefully skip unknown instruction
	if !exists {
		if r.stderr != nil {
			fmt.Fprintf(r.stderr, "# Skipping unknown instruction %s\n", strings.ToUpper(cmd))
		}
		return nil
	}
	ret := method.Func.Call([]reflect.Value{reflect.ValueOf(r.b), reflect.ValueOf(arg)})[0].Interface()
	if ret != nil {
		return ret.(error)
	}
	return nil
}
