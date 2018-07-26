/*
Package buildutil is a wrapper around client package for easier to use build functions.
To be used only in tests for now.
*/
package buildutil

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	controlapi "github.com/moby/buildkit/api/services/control"
	bkclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/sirupsen/logrus"
	"github.com/tonistiigi/fsutil"
)

// BuildKitEnabled returns whether BuildKit was enabled through the DOCKER_BUILDKIT environment variable.
// TODO: Remove this and allow caller to set it.
func BuildKitEnabled() bool {
	return os.Getenv("DOCKER_BUILDKIT") == "1"
}

// BuildResult encapsulates the result of a build.
type BuildResult struct {
	types.BuildResult

	ss bkclient.SolveStatus

	// for legacy builder

	Output []byte
}

// OutputContains returns whether the output from RUN instructions contains a specified search term.
// In legacy builder, this will search within the entire output of the build (not just RUN instructions' output).
func (br *BuildResult) OutputContains(b []byte) bool {
	if !BuildKitEnabled() {
		return bytes.Contains(br.Output, b)
	}
	for _, v := range br.ss.Logs {
		if bytes.Contains(v.Data, b) {
			return true
		}
	}
	return false
}

// CacheHit looks for a Dockerfile step to match with substr and returns whether that step was cached.
// Use randomized strings to match for best quality. Otherwise double check there are no false positives.
func (br *BuildResult) CacheHit(substr string) bool {
	if !BuildKitEnabled() {
		// Parse output by finding first line matching "Using cache", that is below a line matching substr
		s := bufio.NewScanner(bytes.NewReader(br.Output))
		substrMatched := false
		for s.Scan() {
			if !substrMatched {
				if bytes.Contains(s.Bytes(), []byte(substr)) {
					substrMatched = true
				}
			} else {
				if bytes.Equal(s.Bytes(), []byte(" ---> Using cache")) {
					return true
				}
				substrMatched = false
			}
		}
		return false
	}
	for _, v := range br.ss.Vertexes {
		if strings.Contains(v.Name, substr) && v.Cached {
			return true
		}
	}
	return false
}

// BuildInput specifies how to get the build context or Dockerfile.
type BuildInput struct {
	Context    io.Reader
	ContextDir string
	Dockerfile []byte
}

// Build will call client package's ImageBuild and parse the JSONMessage responses into an easy to consume BuildResult.
func Build(c client.APIClient, input BuildInput, options types.ImageBuildOptions) (*BuildResult, error) {
	ctx := context.Background()
	br := &BuildResult{}
	var sess *session.Session

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if BuildKitEnabled() {
		if options.Version != "" && options.Version != types.BuilderBuildKit {
			return br, fmt.Errorf("Conflict: buildkit was enabled but ImageBuildOptions.Version differs: %s", options.Version)
		}
		options.Version = types.BuilderBuildKit
	}

	buildCtx := input.Context

	// if streaming
	if input.Context == nil && options.RemoteContext == "" {
		sess, err := trySession(c, input.ContextDir)
		if err != nil {
			return br, err
		}
		if sess == nil {
			return br, fmt.Errorf("session not supported")
		}

		dirs := make([]filesync.SyncedDir, 0, 1)
		if input.ContextDir != "" {
			var name string
			if BuildKitEnabled() {
				name = "context"
			}
			dirs = append(dirs, filesync.SyncedDir{Name: name, Dir: input.ContextDir, Map: resetUIDAndGID})
		}
		if input.Dockerfile != nil {
			if BuildKitEnabled() {
				dd, err := ioutil.TempDir("", "dockerfiledir")
				if err != nil {
					return br, err
				}
				defer os.RemoveAll(dd)
				if err := ioutil.WriteFile(filepath.Join(dd, "Dockerfile"), input.Dockerfile, 0644); err != nil {
					return br, err
				}
				dirs = append(dirs, filesync.SyncedDir{Name: "dockerfile", Dir: dd})
			} else {
				buildCtx = bytes.NewReader(input.Dockerfile)
			}
		}
		sess.Allow(filesync.NewFSSyncProvider(dirs))

		options.SessionID = sess.ID()
		options.RemoteContext = clientSessionRemote

		go func() {
			if err := sess.Run(ctx, c.DialSession); err != nil {
				logrus.Error(err)
				cancel()
			}
		}()
	}

	if sess != nil {
		defer sess.Close()
	}

	resp, err := c.ImageBuild(ctx, buildCtx, options)
	if err != nil {
		return br, err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	for {
		var jm jsonmessage.JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if err == io.EOF {
				break
			}
			return br, err
		}
		if jm.Error != nil {
			return br, jm.Error
		}
		if jm.ID == "" {
			br.Output = append(br.Output, []byte(jm.Stream)...)
		}
		if jm.Aux != nil {
			switch jm.ID {
			case "moby.buildkit.trace":
				var resp controlapi.StatusResponse
				var dt []byte

				// ignoring all messages that are not understood
				if err := json.Unmarshal(*jm.Aux, &dt); err != nil {
					continue
				}
				if err := (&resp).Unmarshal(dt); err != nil {
					continue
				}

				s := &br.ss
				for _, v := range resp.Vertexes {
					s.Vertexes = append(s.Vertexes, &bkclient.Vertex{
						Digest:    v.Digest,
						Inputs:    v.Inputs,
						Name:      v.Name,
						Started:   v.Started,
						Completed: v.Completed,
						Error:     v.Error,
						Cached:    v.Cached,
					})
				}
				for _, v := range resp.Statuses {
					s.Statuses = append(s.Statuses, &bkclient.VertexStatus{
						ID:        v.ID,
						Vertex:    v.Vertex,
						Name:      v.Name,
						Total:     v.Total,
						Current:   v.Current,
						Timestamp: v.Timestamp,
						Started:   v.Started,
						Completed: v.Completed,
					})
				}
				for _, v := range resp.Logs {
					s.Logs = append(s.Logs, &bkclient.VertexLog{
						Vertex:    v.Vertex,
						Stream:    int(v.Stream),
						Data:      v.Msg,
						Timestamp: v.Timestamp,
					})
				}
			case "", "moby.image.id":
				if err := json.Unmarshal(*jm.Aux, &br.BuildResult); err != nil {
					return br, err
				}
				if br.ID != "" {
					br.ID = br.ID
				}
			}
		}
	}
	return br, nil
}

func resetUIDAndGID(s *fsutil.Stat) bool {
	s.Uid = 0
	s.Gid = 0
	return true
}
