/*
Package buildutil is a wrapper around client package for easier to use build functions.
*/
package buildutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/sirupsen/logrus"
)

// BuildResult encapsulates the result of a build.
type BuildResult struct {
	types.BuildResult
	Output []byte
}

// BuildInput specifies how to get the context or Dockerfile.
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

	buildCtx := input.Context
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
			dirs = append(dirs, filesync.SyncedDir{Dir: input.ContextDir})
		}
		if input.Dockerfile != nil {
			buildCtx = bytes.NewReader(input.Dockerfile)
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
		return nil, err
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
			case "": // legacy builder
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
