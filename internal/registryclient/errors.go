package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/docker/distribution/registry/api/v2"
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
	shortenedResponse := string(e.Response)
	if len(shortenedResponse) > 15 {
		shortenedResponse = shortenedResponse[:12] + "..."
	}
	return fmt.Sprintf("Error parsing HTTP response: %s: %q", e.ParseErr.Error(), shortenedResponse)
}

func parseHTTPErrorResponse(response *http.Response) error {
	var errors v2.Errors
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, &errors); err != nil {
		return &UnexpectedHTTPResponseError{
			ParseErr: err,
			Response: body,
		}
	}
	return &errors
}

func handleErrorResponse(resp *http.Response) error {
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return parseHTTPErrorResponse(resp)
	}
	return &UnexpectedHTTPStatusError{Status: resp.Status}
}
