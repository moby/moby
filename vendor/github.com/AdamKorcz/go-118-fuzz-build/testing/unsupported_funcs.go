package testing

import (
	"testing"
)

func AllocsPerRun(runs int, f func()) (avg float64) {
	panic(unsupportedApi("testing.AllocsPerRun"))
}
func CoverMode() string {
	panic(unsupportedApi("testing.CoverMode"))
}
func Coverage() float64 {
	panic(unsupportedApi("testing.Coverage"))	
}
func Init() {
	panic(unsupportedApi("testing.Init"))

}
func RegisterCover(c testing.Cover) {
	panic(unsupportedApi("testing.RegisterCover"))
}
func RunExamples(matchString func(pat, str string) (bool, error), examples []testing.InternalExample) (ok bool) {
	panic(unsupportedApi("testing.RunExamples"))
}

func RunTests(matchString func(pat, str string) (bool, error), tests []testing.InternalTest) (ok bool) {
	panic(unsupportedApi("testing.RunTests"))
}

func Short() bool {
	panic(unsupportedApi("testing.Short"))
}

func Verbose() bool {
	panic(unsupportedApi("testing.Verbose"))
}

type M struct {}
func (m *M) Run() (code int) {
	panic("testing.M is not support in libFuzzer Mode")
}