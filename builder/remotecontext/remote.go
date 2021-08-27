package remotecontext // import "github.com/docker/docker/builder/remotecontext"

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"

	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/pkg/errors"
)

// When downloading remote contexts, limit the amount (in bytes)
// to be read from the response body in order to detect its Content-Type
const maxPreambleLength = 100

const acceptableRemoteMIME = `(?:application/(?:(?:x\-)?tar|octet\-stream|((?:x\-)?(?:gzip|bzip2?|xz)))|(?:text/plain))`

var mimeRe = regexp.MustCompile(acceptableRemoteMIME)

// downloadRemote context from a url and returns it, along with the parsed content type
func downloadRemote(remoteURL string) (string, io.ReadCloser, error) {
	response, err := GetWithStatusError(remoteURL)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error downloading remote context %s", remoteURL)
	}

	contentType, contextReader, err := inspectResponse(
		response.Header.Get("Content-Type"),
		response.Body,
		response.ContentLength)
	if err != nil {
		response.Body.Close()
		return "", nil, errors.Wrapf(err, "error detecting content type for remote %s", remoteURL)
	}

	return contentType, ioutils.NewReadCloserWrapper(contextReader, response.Body.Close), nil
}

// GetWithStatusError does an http.Get() and returns an error if the
// status code is 4xx or 5xx.
func GetWithStatusError(address string) (resp *http.Response, err error) {
	// #nosec G107
	if resp, err = http.Get(address); err != nil {
		if uerr, ok := err.(*url.Error); ok {
			if derr, ok := uerr.Err.(*net.DNSError); ok && !derr.IsTimeout {
				return nil, errdefs.NotFound(err)
			}
		}
		return nil, errdefs.System(err)
	}
	if resp.StatusCode < 400 {
		return resp, nil
	}
	msg := fmt.Sprintf("failed to GET %s with status %s", address, resp.Status)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, errdefs.System(errors.New(msg + ": error reading body"))
	}

	msg += ": " + string(bytes.TrimSpace(body))
	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, errdefs.NotFound(errors.New(msg))
	case http.StatusBadRequest:
		return nil, errdefs.InvalidParameter(errors.New(msg))
	case http.StatusUnauthorized:
		return nil, errdefs.Unauthorized(errors.New(msg))
	case http.StatusForbidden:
		return nil, errdefs.Forbidden(errors.New(msg))
	}
	return nil, errdefs.Unknown(errors.New(msg))
}

// inspectResponse looks into the http response data at r to determine whether its
// content-type is on the list of acceptable content types for remote build contexts.
// This function returns:
//    - a string representation of the detected content-type
//    - an io.Reader for the response body
//    - an error value which will be non-nil either when something goes wrong while
//      reading bytes from r or when the detected content-type is not acceptable.
func inspectResponse(ct string, r io.Reader, clen int64) (string, io.Reader, error) {
	plen := clen
	if plen <= 0 || plen > maxPreambleLength {
		plen = maxPreambleLength
	}

	preamble := make([]byte, plen)
	rlen, err := r.Read(preamble)
	if rlen == 0 {
		return ct, r, errors.New("empty response")
	}
	if err != nil && err != io.EOF {
		return ct, r, err
	}

	preambleR := bytes.NewReader(preamble[:rlen])
	bodyReader := io.MultiReader(preambleR, r)
	// Some web servers will use application/octet-stream as the default
	// content type for files without an extension (e.g. 'Dockerfile')
	// so if we receive this value we better check for text content
	contentType := ct
	if len(ct) == 0 || ct == mimeTypes.OctetStream {
		contentType, _, err = detectContentType(preamble)
		if err != nil {
			return contentType, bodyReader, err
		}
	}

	contentType = selectAcceptableMIME(contentType)
	var cterr error
	if len(contentType) == 0 {
		cterr = fmt.Errorf("unsupported Content-Type %q", ct)
		contentType = ct
	}

	return contentType, bodyReader, cterr
}

func selectAcceptableMIME(ct string) string {
	return mimeRe.FindString(ct)
}
