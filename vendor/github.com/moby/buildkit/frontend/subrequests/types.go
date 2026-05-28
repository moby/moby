package subrequests

type Request struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Type        RequestType `json:"type"`
	Description string      `json:"description"`
	Opts        []Named     `json:"opts"`
	Inputs      []Named     `json:"inputs"`
	Metadata    []Named     `json:"metadata"`
	Refs        []Named     `json:"refs"`
}

type Named struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type RequestType string

const TypeRPC RequestType = "rpc"
