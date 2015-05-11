package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/docker/distribution/registry/api/v2"
)

// BlobUploadNotFoundError is returned when making a blob upload operation against an
// invalid blob upload location url.
// This may be the result of using a cancelled, completed, or stale upload
// location.
type BlobUploadNotFoundError struct {
	Location string
}

func (e *BlobUploadNotFoundError) Error() string {
	return fmt.Sprintf("No blob upload found at Location: %s", e.Location)
}

// BlobUploadInvalidRangeError is returned when attempting to upload an image
// blob chunk that is out of order.
// This provides the known BlobSize and LastValidRange which can be used to
// resume the upload.
type BlobUploadInvalidRangeError struct {
	Location       string
	LastValidRange int
	BlobSize       int
}

func (e *BlobUploadInvalidRangeError) Error() string {
	return fmt.Sprintf(
		"Invalid range provided for upload at Location: %s. Last Valid Range: %d, Blob Size: %d",
		e.Location, e.LastValidRange, e.BlobSize)
}

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
