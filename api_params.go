package docker

type (
	APITop struct {
		Titles    []string
		Processes [][]string
	}

	APIRmi struct {
		Deleted  string `json:",omitempty"`
		Untagged string `json:",omitempty"`
	}

	APIID struct {
		ID string `json:"Id"`
	}

	APIRun struct {
		ID       string   `json:"Id"`
		Warnings []string `json:",omitempty"`
	}

	APIPort struct {
		PrivatePort int64
		PublicPort  int64
		Type        string
		IP          string
	}

	APIWait struct {
		StatusCode int
	}

	APIAuth struct {
		Status string
	}

	APIImageConfig struct {
		ID string `json:"Id"`
		*Config
	}

	APICopy struct {
		Resource string
		HostPath string
	}
)
