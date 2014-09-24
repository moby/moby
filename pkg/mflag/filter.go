package mflag

func Filter(dst Value, validator func(string) (string, error)) Value {
	return &filter{
		Value:     dst,
		validator: validator,
	}
}

type filter struct {
	Value
	validator func(string) (string, error)
}

func (f *filter) Set(val string) error {
	newval, err := f.validator(val)
	if err != nil {
		return err
	}
	return f.Value.Set(newval)
}

func (f *filter) Get() interface{} {
	if g, ok := f.Value.(Getter); ok {
		return g.Get()
	}
	return nil
}
