package verifier

import (
	"encoding/json"
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/solver/result"
)

const requestOptsKeys = "verifier.requestopts"

const (
	platformsKey = "platform"
	labelsPrefix = "label:"
	keyRequestID = "requestid"
)

type RequestOpts struct {
	Platforms []string
	Labels    map[string]string
	Request   string
}

func CaptureFrontendOpts[T comparable](m map[string]string, res *result.Result[T]) error {
	req := &RequestOpts{}
	if v, ok := m[platformsKey]; ok {
		req.Platforms = strings.Split(v, ",")
	} else {
		req.Platforms = []string{platforms.Format(platforms.Normalize(platforms.DefaultSpec()))}
	}

	req.Labels = map[string]string{}
	for k, v := range m {
		if strings.HasPrefix(k, labelsPrefix) {
			req.Labels[strings.TrimPrefix(k, labelsPrefix)] = v
		}
	}
	req.Request = m[keyRequestID]

	dt, err := json.Marshal(req)
	if err != nil {
		return err
	}
	res.AddMeta(requestOptsKeys, dt)
	return nil
}

func getRequestOpts[T comparable](res *result.Result[T]) (*RequestOpts, error) {
	dt, ok := res.Metadata[requestOptsKeys]
	if !ok {
		return nil, nil
	}
	req := &RequestOpts{}
	if err := json.Unmarshal(dt, req); err != nil {
		return nil, err
	}
	return req, nil
}
