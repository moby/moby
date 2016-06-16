package builder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/urlutil"
)

// When downloading remote contexts, limit the amount (in bytes)
// to be read from the response body in order to detect its Content-Type
const maxPreambleLength = 100

const acceptableRemoteMIME = `(?:application/(?:(?:x\-)?tar|octet\-stream|((?:x\-)?(?:gzip|bzip2?|xz)))|(?:text/plain))`

var mimeRe = regexp.MustCompile(acceptableRemoteMIME)

// MakeRemoteContext downloads a context from remoteURL and returns it.
//
// If contentTypeHandlers is non-nil, then the Content-Type header is read along with a maximum of
// maxPreambleLength bytes from the body to help detecting the MIME type.
// Look at acceptableRemoteMIME for more details.
//
// If a match is found, then the body is sent to the contentType handler and a (potentially compressed) tar stream is expected
// to be returned. If no match is found, it is assumed the body is a tar stream (compressed or not).
// In either case, an (assumed) tar stream is passed to MakeTarSumContext whose result is returned.
func MakeRemoteContext(remoteURL string, contentTypeHandlers map[string]func(io.ReadCloser) (io.ReadCloser, error)) (ModifiableContext, error) {
	f, err := httputils.Download(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("Error downloading remote context %s: %v", remoteURL, err)
	}
	defer f.Body.Close()

	var contextReader io.ReadCloser
	if contentTypeHandlers != nil {
		contentType := f.Header.Get("Content-Type")
		clen := f.ContentLength

		contentType, contextReader, err = inspectResponse(contentType, f.Body, clen)
		if err != nil {
			return nil, fmt.Errorf("Error detecting content type for remote %s: %v", remoteURL, err)
		}
		defer contextReader.Close()

		// This loop tries to find a content-type handler for the detected content-type.
		// If it could not find one from the caller-supplied map, it tries the empty content-type `""`
		// which is interpreted as a fallback handler (usually used for raw tar contexts).
		for _, ct := range []string{contentType, ""} {
			if fn, ok := contentTypeHandlers[ct]; ok {
				defer contextReader.Close()
				if contextReader, err = fn(contextReader); err != nil {
					return nil, err
				}
				break
			}
		}
	}

	// Pass through - this is a pre-packaged context, presumably
	// with a Dockerfile with the right name inside it.
	return MakeTarSumContext(contextReader)
}

// DetectContextFromRemoteURL returns a context and in certain cases the name of the dockerfile to be used
// irrespective of user input.
// progressReader is only used if remoteURL is actually a URL (not empty, and not a Git endpoint).
func DetectContextFromRemoteURL(r io.ReadCloser, remoteURL string, createProgressReader func(in io.ReadCloser) io.ReadCloser) (context ModifiableContext, dockerfileName string, err error) {
	switch {
	case remoteURL == "":
		context, err = MakeTarSumContext(r)
	case urlutil.IsGitURL(remoteURL):
		context, err = MakeGitContext(remoteURL)
	case urlutil.IsURL(remoteURL):
		context, err = MakeRemoteContext(remoteURL, map[string]func(io.ReadCloser) (io.ReadCloser, error){
			httputils.MimeTypes.TextPlain: func(rc io.ReadCloser) (io.ReadCloser, error) {
				dockerfile, err := ioutil.ReadAll(rc)
				if err != nil {
					return nil, err
				}

				// dockerfileName is set to signal that the remote was interpreted as a single Dockerfile, in which case the caller
				// should use dockerfileName as the new name for the Dockerfile, irrespective of any other user input.
				dockerfileName = DefaultDockerfileName

				// TODO: return a context without tarsum
				return archive.Generate(dockerfileName, string(dockerfile))
			},
			// fallback handler (tar context)
			"": func(rc io.ReadCloser) (io.ReadCloser, error) {
				return createProgressReader(rc), nil
			},
		})
	default:
		err = fmt.Errorf("remoteURL (%s) could not be recognized as URL", remoteURL)
	}
	return
}

// inspectResponse looks into the http response data at r to determine whether its
// content-type is on the list of acceptable content types for remote build contexts.
// This function returns:
//    - a string representation of the detected content-type
//    - an io.Reader for the response body
//    - an error value which will be non-nil either when something goes wrong while
//      reading bytes from r or when the detected content-type is not acceptable.
func inspectResponse(ct string, r io.ReadCloser, clen int64) (string, io.ReadCloser, error) {
	plen := clen
	if plen <= 0 || plen > maxPreambleLength {
		plen = maxPreambleLength
	}

	preamble := make([]byte, plen, plen)
	rlen, err := r.Read(preamble)
	if rlen == 0 {
		return ct, r, errors.New("Empty response")
	}
	if err != nil && err != io.EOF {
		return ct, r, err
	}

	preambleR := bytes.NewReader(preamble)
	bodyReader := ioutil.NopCloser(io.MultiReader(preambleR, r))
	// Some web servers will use application/octet-stream as the default
	// content type for files without an extension (e.g. 'Dockerfile')
	// so if we receive this value we better check for text content
	contentType := ct
	if len(ct) == 0 || ct == httputils.MimeTypes.OctetStream {
		contentType, _, err = httputils.DetectContentType(preamble)
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
