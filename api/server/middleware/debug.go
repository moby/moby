package middleware // import "github.com/docker/docker/api/server/middleware"

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/api/server/httpstatus"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/sirupsen/logrus"
)

// DebugRequestMiddleware dumps the request to logger
func DebugRequestMiddleware(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (retErr error) {
		logger := log.G(ctx)

		// Use a variable for fields to prevent overhead of repeatedly
		// calling WithFields.
		fields := log.Fields{
			"module":      "api",
			"method":      r.Method,
			"request-url": r.RequestURI,
			"vars":        vars,
			"status":      http.StatusOK,
		}
		defer func() {
			if retErr != nil {
				fields["error-response"] = retErr
				fields["status"] = httpstatus.FromError(retErr)
			}
			logger.WithFields(fields).Debugf("handling %s request", r.Method)
		}()

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
			// TODO(thaJeztah): is there a better way to detect if we're using JSON-formatted logs?
			if _, ok := logger.Logger.Formatter.(*logrus.JSONFormatter); ok {
				fields["form-data"] = postForm
			} else {
				if data, err := json.Marshal(postForm); err != nil {
					fields["form-data"] = postForm
				} else {
					fields["form-data"] = string(data)
				}
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
