package fluent

//go:generate msgp
type TestMessage struct {
	Foo  string `msg:"foo" json:"foo,omitempty"`
	Hoge string `msg:"hoge" json:"hoge,omitempty"`
}
