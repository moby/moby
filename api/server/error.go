package server

type apiError struct {
	error
	respCode int
}

func newApiError(err error, status int) error {
	return &apiError{err, status}
}
