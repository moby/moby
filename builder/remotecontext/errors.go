package remotecontext

type notFoundError string

func (e notFoundError) Error() string {
	return string(e)
}

func (notFoundError) NotFound() {}

type requestError string

func (e requestError) Error() string {
	return string(e)
}

func (e requestError) InvalidParameter() {}

type unauthorizedError string

func (e unauthorizedError) Error() string {
	return string(e)
}

func (unauthorizedError) Unauthorized() {}

type forbiddenError string

func (e forbiddenError) Error() string {
	return string(e)
}

func (forbiddenError) Forbidden() {}

type dnsError struct {
	cause error
}

func (e dnsError) Error() string {
	return e.cause.Error()
}

func (e dnsError) NotFound() {}

func (e dnsError) Cause() error {
	return e.cause
}

type systemError struct {
	cause error
}

func (e systemError) Error() string {
	return e.cause.Error()
}

func (e systemError) SystemError() {}

func (e systemError) Cause() error {
	return e.cause
}

type unknownError struct {
	cause error
}

func (e unknownError) Error() string {
	return e.cause.Error()
}

func (unknownError) Unknown() {}

func (e unknownError) Cause() error {
	return e.cause
}
