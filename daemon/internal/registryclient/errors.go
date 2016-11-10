package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/client/auth/challenge"
)

// ErrNoErrorsInBody is returned when an HTTP response body parses to an empty
// errcode.Errors slice.
var ErrNoErrorsInBody = errors.New("no error details found in HTTP response body")

// UnexpectedHTTPStatusError is returned when an unexpected HTTP status is
// returned when making a registry api call.
type UnexpectedHTTPStatusError struct {
	Status string
}

func (e *UnexpectedHTTPStatusError) Error() string {
	return fmt.Sprintf("received unexpected HTTP status: %s", e.Status)
}

// UnexpectedHTTPResponseError is returned when an expected HTTP status code
// is returned, but the content was unexpected and failed to be parsed.
type UnexpectedHTTPResponseError struct {
	ParseErr   error
	StatusCode int
	Response   []byte
}

func (e *UnexpectedHTTPResponseError) Error() string {
	return fmt.Sprintf("error parsing HTTP %d response body: %s: %q", e.StatusCode, e.ParseErr.Error(), string(e.Response))
}

// OAuthError is returned when the request could not be authorized
// using the provided oauth token. This could represent a lack of
// permission or invalid token given from a token server.
// See https://tools.ietf.org/html/rfc6750#section-3
type OAuthError struct {
	// ErrorCode is a code defined in https://tools.ietf.org/html/rfc6750#section-3.1
	ErrorCode string

	// Description is the error description associated with the error code
	Description string
}

func (e *OAuthError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("oauth error %q: %s", e.ErrorCode, e.Description)
	}
	return fmt.Sprintf("oauth error %q", e.ErrorCode)
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
		switch statusCode {
		case http.StatusUnauthorized:
			return errcode.ErrorCodeUnauthorized.WithMessage(detailsErr.Details)
		case http.StatusTooManyRequests:
			return errcode.ErrorCodeTooManyRequests.WithMessage(detailsErr.Details)
		default:
			return errcode.ErrorCodeUnknown.WithMessage(detailsErr.Details)
		}
	}

	if err := json.Unmarshal(body, &errors); err != nil {
		return &UnexpectedHTTPResponseError{
			ParseErr:   err,
			StatusCode: statusCode,
			Response:   body,
		}
	}

	if len(errors) == 0 {
		// If there was no error specified in the body, return
		// UnexpectedHTTPResponseError.
		return &UnexpectedHTTPResponseError{
			ParseErr:   ErrNoErrorsInBody,
			StatusCode: statusCode,
			Response:   body,
		}
	}

	return errors
}

// HandleErrorResponse returns error parsed from HTTP response for an
// unsuccessful HTTP response code (in the range 400 - 499 inclusive). An
// UnexpectedHTTPStatusError returned for response code outside of expected
// range.
func HandleErrorResponse(resp *http.Response) error {
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		// Check for OAuth errors within the `WWW-Authenticate` header first
		for _, c := range challenge.ResponseChallenges(resp) {
			if c.Scheme == "bearer" {
				errStr := c.Parameters["error"]
				if errStr != "" {
					return &OAuthError{
						ErrorCode:   errStr,
						Description: c.Parameters["error_description"],
					}
				}
			}
		}
		err := parseHTTPErrorResponse(resp.StatusCode, resp.Body)
		if uErr, ok := err.(*UnexpectedHTTPResponseError); ok && resp.StatusCode == 401 {
			return errcode.ErrorCodeUnauthorized.WithDetail(uErr.Response)
		}
		return err
	}
	return &UnexpectedHTTPStatusError{Status: resp.Status}
}

// SuccessStatus returns true if the argument is a successful HTTP response
// code (in the range 200 - 399 inclusive).
func SuccessStatus(status int) bool {
	return status >= 200 && status <= 399
}
