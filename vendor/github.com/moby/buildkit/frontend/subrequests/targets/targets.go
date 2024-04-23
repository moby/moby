package targets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/subrequests"
	"github.com/moby/buildkit/solver/pb"
)

const RequestTargets = "frontend.targets"

var SubrequestsTargetsDefinition = subrequests.Request{
	Name:        RequestTargets,
	Version:     "1.0.0",
	Type:        subrequests.TypeRPC,
	Description: "List all targets current build supports",
	Opts:        []subrequests.Named{},
	Metadata: []subrequests.Named{
		{Name: "result.json"},
		{Name: "result.txt"},
	},
}

type List struct {
	Targets []Target `json:"targets"`
	Sources [][]byte `json:"sources"`
}

func (l List) ToResult() (*client.Result, error) {
	res := client.NewResult()
	dt, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return nil, err
	}
	res.AddMeta("result.json", dt)

	b := bytes.NewBuffer(nil)
	if err := PrintTargets(dt, b); err != nil {
		return nil, err
	}
	res.AddMeta("result.txt", b.Bytes())

	res.AddMeta("version", []byte(SubrequestsTargetsDefinition.Version))
	return res, nil
}

type Target struct {
	Name        string       `json:"name,omitempty"`
	Default     bool         `json:"default,omitempty"`
	Description string       `json:"description,omitempty"`
	Base        string       `json:"base,omitempty"`
	Platform    string       `json:"platform,omitempty"`
	Location    *pb.Location `json:"location,omitempty"`
}

func PrintTargets(dt []byte, w io.Writer) error {
	var l List

	if err := json.Unmarshal(dt, &l); err != nil {
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	fmt.Fprintf(tw, "TARGET\tDESCRIPTION\n")

	for _, t := range l.Targets {
		name := t.Name
		if name == "" && t.Default {
			name = "(default)"
		} else if t.Default {
			name = fmt.Sprintf("%s (default)", name)
		}
		fmt.Fprintf(tw, "%s\t%s\n", name, t.Description)
	}

	return tw.Flush()
}
