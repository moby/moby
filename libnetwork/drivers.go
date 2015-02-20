package libnetwork

import (
	"fmt"
	"reflect"

	"github.com/docker/libnetwork/pkg/options"
)

type DriverParams options.Generic

var drivers = map[string]struct {
	ctor   interface{}
	config interface{}
}{}

// RegisterNetworkType associates a textual identifier with a way to create a
// new network. It is called by the various network implementations, and used
// upon invokation of the libnetwork.NetNetwork function.
func RegisterNetworkType(name string, ctor interface{}, config interface{}) error {
	if _, ok := drivers[name]; ok {
		return fmt.Errorf("a driver for network type %q is already registed", name)
	}
	drivers[name] = struct {
		ctor   interface{}
		config interface{}
	}{ctor, config}
	return nil
}

func createNetwork(name string, generic DriverParams) (Network, error) {
	d, ok := drivers[name]
	if !ok {
		return nil, fmt.Errorf("unknown driver %q", name)
	}

	config, err := options.GenerateFromModel(options.Generic(generic), d.config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate driver config: %v", err)
	}

	arg := []reflect.Value{reflect.ValueOf(config)}
	res := reflect.ValueOf(d.ctor).Call(arg)
	return makeCreateResult(res)
}

func makeCreateResult(res []reflect.Value) (net Network, err error) {
	if !res[0].IsNil() {
		net = res[0].Interface().(Network)
	}
	if !res[1].IsNil() {
		err = res[1].Interface().(error)
	}
	return
}
