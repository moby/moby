package middleware // import "github.com/docker/docker/api/server/middleware"

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/pkg/ioutils"
)

// DebugRequestMiddleware dumps the request to logger
func DebugRequestMiddleware(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		log.G(ctx).Debugf("Calling %s %s", r.Method, r.RequestURI)

		if r.Method != http.MethodPost {
			return handler(ctx, w, r, vars)
		}
		if err := httputils.CheckForJSON(r); err != nil {
			return handler(ctx, w, r, vars)
		}
		maxBodySize := 4096 // 4KB
		if r.ContentLength > int64(maxBodySize) {
			return handler(ctx, w, r, vars)
		}

		body := r.Body
		bufReader := bufio.NewReaderSize(body, maxBodySize)
		r.Body = ioutils.NewReadCloserWrapper(bufReader, func() error { return body.Close() })

		b, err := bufReader.Peek(maxBodySize)
		if err != io.EOF {
			// either there was an error reading, or the buffer is full (in which case the request is too large)
			return handler(ctx, w, r, vars)
		}

		var postForm map[string]interface{}
		if err := json.Unmarshal(b, &postForm); err == nil {
			maskSecretKeys(postForm)
			formStr, errMarshal := json.Marshal(postForm)
			if errMarshal == nil {
				log.G(ctx).Debugf("form data: %s", string(formStr))
			} else {
				log.G(ctx).Debugf("form data: %q", postForm)
			}
		}

		return handler(ctx, w, r, vars)
	}
}

func maskSecretKeys(inp interface{}) {
	if arr, ok := inp.([]interface{}); ok {
		for _, f := range arr {
			maskSecretKeys(f)
		}
		return
	}

	if form, ok := inp.(map[string]interface{}); ok {
		scrub := []string{
			// Note: The Data field contains the base64-encoded secret in 'secret'
			// and 'config' create and update requests. Currently, no other POST
			// API endpoints use a data field, so we scrub this field unconditionally.
			// Change this handling to be conditional if a new endpoint is added
			// in future where this field should not be scrubbed.
			"data",
			"jointoken",
			"password",
			"secret",
			"signingcakey",
			"unlockkey",
		}
	loop0:
		for k, v := range form {
			for _, m := range scrub {
				if strings.EqualFold(m, k) {
					form[k] = "*****"
					continue loop0
				}
			}
			maskSecretKeys(v)
		}
	}
}
