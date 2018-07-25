/*
Package buildutil is a wrapper around client package for easier to use build functions.
*/
package buildutil

import (
	"context"
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
)

// BuildResult encapsulates the result of a build.
type BuildResult struct {
	types.BuildResult
	Output []byte
}

// BuildInput specifies how to get the context.
type BuildInput struct {
	Context io.Reader
}

// Build will call client package's ImageBuild and parse the JSONMessage responses into an easy to consume BuildResult.
func Build(c client.APIClient, input BuildInput, options types.ImageBuildOptions) (*BuildResult, error) {
	ctx := context.Background()
	br := &BuildResult{}
	resp, err := c.ImageBuild(ctx, input.Context, options)
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
