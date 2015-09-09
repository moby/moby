package httputils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/docker/api"
)

// CheckForJSON makes sure that the request's Content-Type is application/json.
func CheckForJSON(r *http.Request) error {
	ct := r.Header.Get("Content-Type")

	// No Content-Type header is ok as long as there's no Body
	if ct == "" {
		if r.Body == nil || r.ContentLength == 0 {
			return nil
		}
	}

	// Otherwise it better be json
	if api.MatchesContentType(ct, "application/json") {
		return nil
	}
	return fmt.Errorf("Content-Type specified (%s) must be 'application/json'", ct)
}

// ParseForm ensures that the form in the request is properly parsed.
// If we don't do this, POST method without Content-type (even with empty body) will fail
func ParseForm(r *http.Request) error {
	if r == nil {
		return nil
	}
	if err := r.ParseForm(); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return err
	}
	return nil
}

// ParseMultipartForm ensures that the form in the request is parsed as a multipart form.
func ParseMultipartForm(r *http.Request) error {
	if err := r.ParseMultipartForm(4096); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return err
	}
	return nil
}

// HTTPError transforms an internal error into a valid http error for the api.
// It also writes the error in the response.
func HTTPError(w http.ResponseWriter, err error) {
	if err == nil || w == nil {
		logrus.WithFields(logrus.Fields{"error": err, "writer": w}).Error("unexpected HTTP error handling")
		return
	}

	statusCode := http.StatusInternalServerError
	errMsg := err.Error()

	// Based on the type of error we get we need to process things
	// slightly differently to extract the error message.
	// In the 'errcode.*' cases there are two different type of
	// error that could be returned. errocode.ErrorCode is the base
	// type of error object - it is just an 'int' that can then be
	// used as the look-up key to find the message. errorcode.Error
	// extends errorcode.Error by adding error-instance specific
	// data, like 'details' or variable strings to be inserted into
	// the message.
	//
	// Ideally, we should just be able to call err.Error() for all
	// cases but the errcode package doesn't support that yet.
	//
	// Additionally, in both errcode cases, there might be an http
	// status code associated with it, and if so use it.
	switch err.(type) {
	case errcode.ErrorCode:
		daError, _ := err.(errcode.ErrorCode)
		statusCode = daError.Descriptor().HTTPStatusCode
		errMsg = daError.Message()

	case errcode.Error:
		// For reference, if you're looking for a particular error
		// then you can do something like :
		//   import ( derr "github.com/docker/docker/api/errors" )
		//   if daError.ErrorCode() == derr.ErrorCodeNoSuchContainer { ... }

		daError, _ := err.(errcode.Error)
		statusCode = daError.ErrorCode().Descriptor().HTTPStatusCode
		errMsg = daError.Message

	default:
		// This part of will be removed once we've
		// converted everything over to use the errcode package

		// FIXME: this is brittle and should not be necessary.
		// If we need to differentiate between different possible error types,
		// we should create appropriate error types with clearly defined meaning
		errStr := strings.ToLower(err.Error())
		for keyword, status := range map[string]int{
			"not found":             http.StatusNotFound,
			"no such":               http.StatusNotFound,
			"bad parameter":         http.StatusBadRequest,
			"conflict":              http.StatusConflict,
			"impossible":            http.StatusNotAcceptable,
			"wrong login/password":  http.StatusUnauthorized,
			"hasn't been activated": http.StatusForbidden,
		} {
			if strings.Contains(errStr, keyword) {
				statusCode = status
				break
			}
		}
	}

	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}

	logrus.WithFields(logrus.Fields{"statusCode": statusCode, "err": err}).Error("HTTP Error")
	http.Error(w, errMsg, statusCode)
}

// WriteJSON writes the value v to the http response stream as json with standard
// json encoding.
func WriteJSON(w http.ResponseWriter, code int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(v)
}
