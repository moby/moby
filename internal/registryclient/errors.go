package client

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

// UnexpectedHTTPStatusError is returned when an unexpected HTTP status is
// returned when making a registry api call.
type UnexpectedHTTPStatusError struct {
	Status string
}

func (e *UnexpectedHTTPStatusError) Error() string {
	return fmt.Sprintf("Received unexpected HTTP status: %s", e.Status)
}

// UnexpectedHTTPResponseError is returned when an expected HTTP status code
// is returned, but the content was unexpected and failed to be parsed.
type UnexpectedHTTPResponseError struct {
	ParseErr error
	Response []byte
}

func (e *UnexpectedHTTPResponseError) Error() string {
	return fmt.Sprintf("Error parsing HTTP response: %s: %q", e.ParseErr.Error(), string(e.Response))
}

func parseHTTPErrorResponse(statusCode int, r io.Reader) error {
	var errors errcode.Errors
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	// For backward compatibility, handle irregularly formatted
	// messages that contain a "details" field.
	var detailsErr struct {
		Details string `json:"details"`
	}
	err = json.Unmarshal(body, &detailsErr)
	if err == nil && detailsErr.Details != "" {
		if statusCode == http.StatusUnauthorized {
			return errcode.ErrorCodeUnauthorized.WithMessage(detailsErr.Details)
		}
		return errcode.ErrorCodeUnknown.WithMessage(detailsErr.Details)
	}

	if err := json.Unmarshal(body, &errors); err != nil {
		return &UnexpectedHTTPResponseError{
			ParseErr: err,
			Response: body,
		}
	}
	return errors
}

// HandleErrorResponse returns error parsed from HTTP response for an
// unsuccessful HTTP response code (in the range 400 - 499 inclusive). An
// UnexpectedHTTPStatusError returned for response code outside of expected
// range.
func HandleErrorResponse(resp *http.Response) error {
	if resp.StatusCode == 401 {
		err := parseHTTPErrorResponse(resp.StatusCode, resp.Body)
		if uErr, ok := err.(*UnexpectedHTTPResponseError); ok {
			return errcode.ErrorCodeUnauthorized.WithDetail(uErr.Response)
		}
		return err
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return parseHTTPErrorResponse(resp.StatusCode, resp.Body)
	}
	return &UnexpectedHTTPStatusError{Status: resp.Status}
}

// SuccessStatus returns true if the argument is a successful HTTP response
// code (in the range 200 - 399 inclusive).
func SuccessStatus(status int) bool {
	return status >= 200 && status <= 399
}
