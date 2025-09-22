package opts

type Option[T any] interface {
	Apply(opt *T) error
}

type OptionFunc[T any] func(opt *T) error

func (f OptionFunc[T]) Apply(opt *T) error {
	return f(opt)
}
