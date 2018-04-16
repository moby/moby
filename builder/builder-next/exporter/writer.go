package containerimage

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"time"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/system"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	emptyGZLayer = digest.Digest("sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1")
)

func emptyImageConfig() ([]byte, error) {
	img := ocispec.Image{
		Architecture: runtime.GOARCH,
		OS:           runtime.GOOS,
	}
	img.RootFS.Type = "layers"
	img.Config.WorkingDir = "/"
	img.Config.Env = []string{"PATH=" + system.DefaultPathEnv}
	dt, err := json.Marshal(img)
	return dt, errors.Wrap(err, "failed to create empty image config")
}

func parseHistoryFromConfig(dt []byte) ([]ocispec.History, error) {
	var config struct {
		History []ocispec.History
	}
	if err := json.Unmarshal(dt, &config); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal history from config")
	}
	return config.History, nil
}

func patchImageConfig(dt []byte, dps []digest.Digest, history []ocispec.History) ([]byte, error) {
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(dt, &m); err != nil {
		return nil, errors.Wrap(err, "failed to parse image config for patch")
	}

	var rootFS ocispec.RootFS
	rootFS.Type = "layers"
	for _, dp := range dps {
		rootFS.DiffIDs = append(rootFS.DiffIDs, dp)
	}
	dt, err := json.Marshal(rootFS)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal rootfs")
	}
	m["rootfs"] = dt

	dt, err = json.Marshal(history)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal history")
	}
	m["history"] = dt

	now := time.Now()
	dt, err = json.Marshal(&now)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal creation time")
	}
	m["created"] = dt

	dt, err = json.Marshal(m)
	return dt, errors.Wrap(err, "failed to marshal config after patch")
}

func normalizeLayersAndHistory(diffs []digest.Digest, history []ocispec.History, ref cache.ImmutableRef) ([]digest.Digest, []ocispec.History) {
	var historyLayers int
	for _, h := range history {
		if !h.EmptyLayer {
			historyLayers += 1
		}
	}
	if historyLayers > len(diffs) {
		// this case shouldn't happen but if it does force set history layers empty
		// from the bottom
		logrus.Warn("invalid image config with unaccounted layers")
		historyCopy := make([]ocispec.History, 0, len(history))
		var l int
		for _, h := range history {
			if l >= len(diffs) {
				h.EmptyLayer = true
			}
			if !h.EmptyLayer {
				l++
			}
			historyCopy = append(historyCopy, h)
		}
		history = historyCopy
	}

	if len(diffs) > historyLayers {
		// some history items are missing. add them based on the ref metadata
		for _, msg := range getRefDesciptions(ref, len(diffs)-historyLayers) {
			tm := time.Now().UTC()
			history = append(history, ocispec.History{
				Created:   &tm,
				CreatedBy: msg,
				Comment:   "buildkit.exporter.image.v0",
			})
		}
	}

	// var layerIndex int
	// for i, h := range history {
	// 	if !h.EmptyLayer {
	// 		if diffs[layerIndex] == emptyGZLayer { // TODO: fixme
	// 			h.EmptyLayer = true
	// 			diffs = append(diffs[:layerIndex], diffs[layerIndex+1:]...)
	// 		} else {
	// 			layerIndex++
	// 		}
	// 	}
	// 	history[i] = h
	// }

	return diffs, history
}

func getRefDesciptions(ref cache.ImmutableRef, limit int) []string {
	if limit <= 0 {
		return nil
	}
	defaultMsg := "created by buildkit" // shouldn't happen but don't fail build
	if ref == nil {
		strings.Repeat(defaultMsg, limit)
	}
	descr := cache.GetDescription(ref.Metadata())
	if descr == "" {
		descr = defaultMsg
	}
	p := ref.Parent()
	if p != nil {
		defer p.Release(context.TODO())
	}
	return append(getRefDesciptions(p, limit-1), descr)
}

func oneOffProgress(ctx context.Context, id string) func(err error) error {
	pw, _, _ := progress.FromContext(ctx)
	now := time.Now()
	st := progress.Status{
		Started: &now,
	}
	pw.Write(id, st)
	return func(err error) error {
		// TODO: set error on status
		now := time.Now()
		st.Completed = &now
		pw.Write(id, st)
		pw.Close()
		return err
	}
}
