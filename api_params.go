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

	APIContainers struct {
		ID         string `json:"Id"`
		Image      string
		Command    string
		Created    int64
		Status     string
		Ports      []APIPort
		SizeRw     int64
		SizeRootFs int64
		Names      []string
	}

	APIContainersOld struct {
		ID         string `json:"Id"`
		Image      string
		Command    string
		Created    int64
		Status     string
		Ports      string
		SizeRw     int64
		SizeRootFs int64
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

func (api APIContainers) ToLegacy() *APIContainersOld {
	return &APIContainersOld{
		ID:         api.ID,
		Image:      api.Image,
		Command:    api.Command,
		Created:    api.Created,
		Status:     api.Status,
		Ports:      displayablePorts(api.Ports),
		SizeRw:     api.SizeRw,
		SizeRootFs: api.SizeRootFs,
	}
}
