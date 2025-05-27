package testing

import (
	"fmt"
	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"os"
	"reflect"
)

type F struct {
	Data     []byte
	T        *T
	FuzzFunc func(*T, any)
}

func (f *F) CleanupTempDirs() {
	f.T.CleanupTempDirs()
}

func (f *F) Add(args ...any)                   {}
func (c *F) Cleanup(f func())                  {}
func (c *F) Error(args ...any)                 {}
func (c *F) Errorf(format string, args ...any) {}
func (f *F) Fail()                             {}
func (c *F) FailNow()                          {}
func (c *F) Failed() bool                      { return false }
func (c *F) Fatal(args ...any)                 {}
func (c *F) Fatalf(format string, args ...any) {}
func (f *F) Fuzz(ff any) {
	// we are assuming that ff is a func.
	// TODO: Add a check for UX purposes

	fn := reflect.ValueOf(ff)
	fnType := fn.Type()
	var types []reflect.Type
	for i := 1; i < fnType.NumIn(); i++ {
		t := fnType.In(i)

		types = append(types, t)
	}
	args := []reflect.Value{reflect.ValueOf(f.T)}
	fuzzConsumer := fuzz.NewConsumer(f.Data)
	for _, v := range types {
		//fmt.Printf("arg %v\n", v)
		newElem := reflect.New(v).Elem()
		switch v.String() {
		case "[]uint8":
			b, err := fuzzConsumer.GetBytes()
			if err != nil {
				return
			}
			newElem.SetBytes(b)
		case "string":
			s, err := fuzzConsumer.GetString()
			if err != nil {
				return
			}
			newElem.SetString(s)
		case "int":
			randInt, err := fuzzConsumer.GetUint64()
			if err != nil {
				return
			}
			newElem.SetInt(int64(int(randInt)))
		case "int8":
			randInt, err := fuzzConsumer.GetByte()
			if err != nil {
				return
			}
			newElem.SetInt(int64(randInt))
		case "int16":
			randInt, err := fuzzConsumer.GetUint16()
			if err != nil {
				return
			}
			newElem.SetInt(int64(randInt))
		case "int32":
			randInt, err := fuzzConsumer.GetUint32()
			if err != nil {
				return
			}
			newElem.SetInt(int64(int32(randInt)))
		case "int64":
			randInt, err := fuzzConsumer.GetUint64()
			if err != nil {
				return
			}
			newElem.SetInt(int64(randInt))
		case "uint":
			randInt, err := fuzzConsumer.GetUint64()
			if err != nil {
				return
			}
			newElem.SetUint(uint64(uint(randInt)))
		case "uint8":
			randInt, err := fuzzConsumer.GetByte()
			if err != nil {
				return
			}
			newElem.SetUint(uint64(randInt))
		case "uint16":
			randInt, err := fuzzConsumer.GetUint16()
			if err != nil {
				return
			}
			newElem.SetUint(uint64(randInt))
		case "uint32":
			randInt, err := fuzzConsumer.GetUint32()
			if err != nil {
				return
			}
			newElem.SetUint(uint64(randInt))
		case "uint64":
			randInt, err := fuzzConsumer.GetUint64()
			if err != nil {
				return
			}
			newElem.SetUint(uint64(randInt))
		case "rune":
			randRune, err := fuzzConsumer.GetRune()
			if err != nil {
				return
			}
			newElem.Set(reflect.ValueOf(randRune))
		case "float32":
			randFloat, err := fuzzConsumer.GetFloat32()
			if err != nil {
				return
			}
			newElem.Set(reflect.ValueOf(randFloat))
		case "float64":
			randFloat, err := fuzzConsumer.GetFloat64()
			if err != nil {
				return
			}
			newElem.Set(reflect.ValueOf(randFloat))
		case "bool":
			randBool, err := fuzzConsumer.GetBool()
			if err != nil {
				return
			}
			newElem.Set(reflect.ValueOf(randBool))
		default:
			panic(fmt.Sprintf("unsupported type: %s", v.String()))
		}
		args = append(args, newElem)

	}
	fn.Call(args)
}
func (f *F) Helper() {}
func (c *F) Log(args ...any) {
	fmt.Print(args...)
}
func (c *F) Logf(format string, args ...any) {
	fmt.Println(fmt.Sprintf(format, args...))
}
func (c *F) Name() string             { return "libFuzzer" }
func (c *F) Setenv(key, value string) {}
func (c *F) Skip(args ...any) {
	panic("GO-FUZZ-BUILD-PANIC")
}
func (c *F) SkipNow() {
	panic("GO-FUZZ-BUILD-PANIC")
}
func (c *F) Skipf(format string, args ...any) {
	panic("GO-FUZZ-BUILD-PANIC")
}
func (f *F) Skipped() bool { return false }

func (f *F) TempDir() string {
	dir, err := os.MkdirTemp("", "fuzzdir-")
	if err != nil {
		panic(err)
	}
	f.T.TempDirs = append(f.T.TempDirs, dir)

	return dir
}
