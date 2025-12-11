package convertllb

import (
	"encoding/json"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/subrequests"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	"google.golang.org/protobuf/encoding/protojson"
)

const RequestConvertLLB = "frontend.convertllb"

var SubrequestConvertLLBDefinition = subrequests.Request{
	Name:        RequestConvertLLB,
	Version:     "0.1.0",
	Type:        subrequests.TypeRPC,
	Description: "Convert Dockerfile to LLB",
	Opts:        []subrequests.Named{},
	Metadata: []subrequests.Named{
		{Name: "result.json"},
	},
}

type Result struct {
	Def      map[digest.Digest]*pb.Op         `json:"def"`
	Metadata map[digest.Digest]llb.OpMetadata `json:"metadata"`
	Source   *pb.Source                       `json:"source"`
}

func (result *Result) ToResult() (*client.Result, error) {
	res := client.NewResult()
	dt, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	res.AddMeta("result.json", dt)

	res.AddMeta("version", []byte(SubrequestConvertLLBDefinition.Version))
	return res, nil
}

func (result *Result) MarshalJSON() ([]byte, error) {
	var jsonResult struct {
		Def      map[digest.Digest]json.RawMessage `json:"def"`
		Metadata map[digest.Digest]llb.OpMetadata  `json:"metadata"`
		Source   json.RawMessage                   `json:"source"`
	}
	jsonResult.Def = make(map[digest.Digest]json.RawMessage, len(result.Def))
	for dgst, op := range result.Def {
		dt, err := protojson.Marshal(op)
		if err != nil {
			return nil, err
		}
		jsonResult.Def[dgst] = dt
	}
	jsonResult.Metadata = result.Metadata

	src, err := protojson.Marshal(result.Source)
	if err != nil {
		return nil, err
	}
	jsonResult.Source = src
	return json.Marshal(&jsonResult)
}
