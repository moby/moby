package outline

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

const RequestSubrequestsOutline = "frontend.outline"

var SubrequestsOutlineDefinition = subrequests.Request{
	Name:        RequestSubrequestsOutline,
	Version:     "1.0.0",
	Type:        subrequests.TypeRPC,
	Description: "List all parameters current build target supports",
	Opts: []subrequests.Named{
		{
			Name:        "target",
			Description: "Target build stage",
		},
	},
	Metadata: []subrequests.Named{
		{Name: "result.json"},
		{Name: "result.txt"},
	},
}

type Outline struct {
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	Args        []Arg        `json:"args,omitempty"`
	Secrets     []Secret     `json:"secrets,omitempty"`
	SSH         []SSH        `json:"ssh,omitempty"`
	Cache       []CacheMount `json:"cache,omitempty"`
	Sources     [][]byte     `json:"sources,omitempty"`
}

func (o Outline) ToResult() (*client.Result, error) {
	res := client.NewResult()
	dt, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return nil, err
	}
	res.AddMeta("result.json", dt)

	b := bytes.NewBuffer(nil)
	if err := PrintOutline(dt, b); err != nil {
		return nil, err
	}
	res.AddMeta("result.txt", b.Bytes())

	res.AddMeta("version", []byte(SubrequestsOutlineDefinition.Version))
	return res, nil
}

type Arg struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Value       string       `json:"value,omitempty"`
	Location    *pb.Location `json:"location,omitempty"`
}

type Secret struct {
	Name     string       `json:"name"`
	Required bool         `json:"required,omitempty"`
	Location *pb.Location `json:"location,omitempty"`
}

type SSH struct {
	Name     string       `json:"name"`
	Required bool         `json:"required,omitempty"`
	Location *pb.Location `json:"location,omitempty"`
}

type CacheMount struct {
	ID       string       `json:"ID"`
	Location *pb.Location `json:"location,omitempty"`
}

func PrintOutline(dt []byte, w io.Writer) error {
	var o Outline

	if err := json.Unmarshal(dt, &o); err != nil {
		return err
	}

	if o.Name != "" || o.Description != "" {
		tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
		name := o.Name
		if o.Name == "" {
			name = "(default)"
		}
		fmt.Fprintf(tw, "TARGET:\t%s\n", name)
		if o.Description != "" {
			fmt.Fprintf(tw, "DESCRIPTION:\t%s\n", o.Description)
		}
		tw.Flush()
		fmt.Fprintln(tw)
	}

	if len(o.Args) > 0 {
		tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
		fmt.Fprintf(tw, "BUILD ARG\tVALUE\tDESCRIPTION\n")
		for _, a := range o.Args {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", a.Name, a.Value, a.Description)
		}
		tw.Flush()
		fmt.Fprintln(tw)
	}

	if len(o.Secrets) > 0 {
		tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
		fmt.Fprintf(tw, "SECRET\tREQUIRED\n")
		for _, s := range o.Secrets {
			b := ""
			if s.Required {
				b = "true"
			}
			fmt.Fprintf(tw, "%s\t%s\n", s.Name, b)
		}
		tw.Flush()
		fmt.Fprintln(tw)
	}

	if len(o.SSH) > 0 {
		tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
		fmt.Fprintf(tw, "SSH\tREQUIRED\n")
		for _, s := range o.SSH {
			b := ""
			if s.Required {
				b = "true"
			}
			fmt.Fprintf(tw, "%s\t%s\n", s.Name, b)
		}
		tw.Flush()
		fmt.Fprintln(tw)
	}

	return nil
}
