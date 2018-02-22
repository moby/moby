package funker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
)

// Handle a Funker function.
func Handle(handler interface{}) error {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()
	if handlerType.Kind() != reflect.Func || handlerType.NumIn() != 1 || handlerType.NumOut() != 1 {
		return fmt.Errorf("Handler must be a function with a single parameter and single return value.")
	}
	argsValue := reflect.New(handlerType.In(0))

	listener, err := net.Listen("tcp", ":9999")
	if err != nil {
		return err
	}
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	// We close listener, because we only allow single request.
	// Note that TCP "backlog" cannot be used for that purpose.
	// http://www.perlmonks.org/?node_id=940662
	if err = listener.Close(); err != nil {
		return err
	}
	argsJSON, err := ioutil.ReadAll(conn)
	if err != nil {
		return err
	}
	err = json.Unmarshal(argsJSON, argsValue.Interface())
	if err != nil {
		return err
	}

	ret := handlerValue.Call([]reflect.Value{argsValue.Elem()})[0].Interface()
	retJSON, err := json.Marshal(ret)
	if err != nil {
		return err
	}

	if _, err = conn.Write(retJSON); err != nil {
		return err
	}

	return conn.Close()
}
