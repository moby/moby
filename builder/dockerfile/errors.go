package dockerfile

type validationError struct {
	err error
}

func (e validationError) Error() string {
	return e.err.Error()
}

func (e validationError) InvalidParameter() {}

func (e validationError) Cause() error {
	return e.err
}
