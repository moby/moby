package gofuzzheaders

import (
	"fmt"
	"reflect"
)

type Continue struct {
	F *ConsumeFuzzer
}

func (f *ConsumeFuzzer) AddFuncs(fuzzFuncs []interface{}) {
	for i := range fuzzFuncs {
		v := reflect.ValueOf(fuzzFuncs[i])
		if v.Kind() != reflect.Func {
			panic("Need only funcs!")
		}
		t := v.Type()
		if t.NumIn() != 2 || t.NumOut() != 1 {
			fmt.Println(t.NumIn(), t.NumOut())

			panic("Need 2 in and 1 out params. In must be the type. Out must be an error")
		}
		argT := t.In(0)
		switch argT.Kind() {
		case reflect.Ptr, reflect.Map:
		default:
			panic("fuzzFunc must take pointer or map type")
		}
		if t.In(1) != reflect.TypeOf(Continue{}) {
			panic("fuzzFunc's second parameter must be type Continue")
		}
		f.Funcs[argT] = v
	}
}

func (f *ConsumeFuzzer) GenerateWithCustom(targetStruct interface{}) error {
	e := reflect.ValueOf(targetStruct).Elem()
	return f.fuzzStruct(e, true)
}

func (c Continue) GenerateStruct(targetStruct interface{}) error {
	return c.F.GenerateStruct(targetStruct)
}

func (c Continue) GenerateStructWithCustom(targetStruct interface{}) error {
	return c.F.GenerateWithCustom(targetStruct)
}
