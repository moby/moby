package imageutil

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/containerd/containerd/remotes"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func readSchema1Config(ctx context.Context, ref string, desc ocispecs.Descriptor, fetcher remotes.Fetcher, cache ContentCache) (digest.Digest, []byte, error) {
	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return "", nil, err
	}
	defer rc.Close()
	dt, err := io.ReadAll(rc)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to fetch schema1 manifest")
	}
	dt, err = convertSchema1ConfigMeta(dt)
	if err != nil {
		return "", nil, err
	}
	return desc.Digest, dt, nil
}

func convertSchema1ConfigMeta(in []byte) ([]byte, error) {
	type history struct {
		V1Compatibility string `json:"v1Compatibility"`
	}
	var m struct {
		History []history `json:"history"`
	}
	if err := json.Unmarshal(in, &m); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal schema1 manifest")
	}
	if len(m.History) == 0 {
		return nil, errors.Errorf("invalid schema1 manifest")
	}

	var img ocispecs.Image
	if err := json.Unmarshal([]byte(m.History[0].V1Compatibility), &img); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal image from schema 1 history")
	}

	img.RootFS = ocispecs.RootFS{
		Type: "layers", // filled in by exporter
	}
	img.History = make([]ocispecs.History, len(m.History))

	for i := range m.History {
		var h v1History
		if err := json.Unmarshal([]byte(m.History[i].V1Compatibility), &h); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal history")
		}
		img.History[len(m.History)-i-1] = ocispecs.History{
			Author:     h.Author,
			Comment:    h.Comment,
			Created:    &h.Created,
			CreatedBy:  strings.Join(h.ContainerConfig.Cmd, " "),
			EmptyLayer: (h.ThrowAway != nil && *h.ThrowAway) || (h.Size != nil && *h.Size == 0),
		}
	}

	dt, err := json.MarshalIndent(img, "", "   ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal schema1 config")
	}
	return dt, nil
}

type v1History struct {
	Author          string    `json:"author,omitempty"`
	Created         time.Time `json:"created"`
	Comment         string    `json:"comment,omitempty"`
	ThrowAway       *bool     `json:"throwaway,omitempty"`
	Size            *int      `json:"Size,omitempty"` // used before ThrowAway field
	ContainerConfig struct {
		Cmd []string `json:"Cmd,omitempty"`
	} `json:"container_config,omitempty"`
}
