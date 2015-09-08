package foo

type wobble struct {
	Some      string
	Val       string
	Inception *wobble
}

// Fooer is an empty interface used for tests.
type Fooer interface{}

// Fooer2 is an interface used for tests.
type Fooer2 interface {
	Foo()
}

// Fooer3 is an interface used for tests.
type Fooer3 interface {
	Foo()
	Bar(a string)
	Baz(a string) (err error)
	Qux(a, b string) (val string, err error)
	Wobble() (w *wobble)
	Wiggle() (w wobble)
}

// Fooer4 is an interface used for tests.
type Fooer4 interface {
	Foo() error
}

// Bar is an interface used for tests.
type Bar interface {
	Boo(a string, b string) (s string, err error)
}

// Fooer5 is an interface used for tests.
type Fooer5 interface {
	Foo()
	Bar
}
