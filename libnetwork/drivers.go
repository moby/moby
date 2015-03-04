package libnetwork

import (
	"fmt"
	"reflect"

	"github.com/docker/libnetwork/pkg/options"
)

// DriverParams are a generic structure to hold driver specific settings.
type DriverParams options.Generic

var drivers = map[string]struct {
	creatorFn  interface{}
	creatorArg interface{}
}{}

// RegisterNetworkType associates a textual identifier with a way to create a
// new network. It is called by the various network implementations, and used
// upon invokation of the libnetwork.NetNetwork function.
//
// creatorFn must be of type func (creatorArgType) (Network, error), where
// createArgType is the type of the creatorArg argument.
//
// For example:
//
//    func CreateTestNetwork(name string, config *TestNetworkConfig()) (Network, error) {
//    }
//
//    func init() {
//        RegisterNetworkType("test", CreateTestNetwork, &TestNetworkConfig{})
//    }
//
func RegisterNetworkType(name string, creatorFn interface{}, creatorArg interface{}) error {
	// Validate the creator function signature.
	ctorArg := []reflect.Type{reflect.TypeOf((*string)(nil)), reflect.TypeOf(creatorArg)}
	ctorRet := []reflect.Type{reflect.TypeOf((*Network)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()}
	if err := validateFunctionSignature(creatorFn, ctorArg, ctorRet); err != nil {
		sig := fmt.Sprintf("func (%s) (Network, error)", ctorArg[0].Name)
		return fmt.Errorf("invalid signature for %q creator function (expected %s)", name, sig)
	}

	// Store the new driver information to invoke at creation time.
	if _, ok := drivers[name]; ok {
		return fmt.Errorf("a driver for network type %q is already registed", name)
	}
	drivers[name] = struct {
		creatorFn  interface{}
		creatorArg interface{}
	}{creatorFn, creatorArg}
	return nil
}

func createNetwork(networkType, name string, generic DriverParams) (Network, error) {
	d, ok := drivers[networkType]
	if !ok {
		return nil, fmt.Errorf("unknown driver %q", networkType)
	}

	config, err := options.GenerateFromModel(options.Generic(generic), d.creatorArg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate driver config: %v", err)
	}

	arg := []reflect.Value{reflect.ValueOf(name), reflect.ValueOf(config)}
	res := reflect.ValueOf(d.creatorFn).Call(arg)
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

func validateFunctionSignature(fn interface{}, params []reflect.Type, returns []reflect.Type) error {
	// Valid that argument is a function.
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return fmt.Errorf("argument is %s, not function", fnType.Name)
	}

	// Vaidate arguments numbers and types.
	if fnType.NumIn() != len(params) {
		return fmt.Errorf("expected function with %d arguments, got %d", len(params), fnType.NumIn())
	}
	for i, argType := range params {
		if argType != fnType.In(i) {
			return fmt.Errorf("argument %d type should be %s, got %s", i, argType.Name, fnType.In(i).Name)
		}
	}

	// Validate return values numbers and types.
	if fnType.NumOut() != len(returns) {
		return fmt.Errorf("expected function with %d return values, got %d", len(params), fnType.NumIn())
	}
	for i, retType := range returns {
		if retType != fnType.Out(i) {
			return fmt.Errorf("return value %d type should be %s, got %s", i, retType.Name, fnType.Out(i).Name)
		}
	}

	return nil
}
