package docker

type APIHistory struct {
	ID        string   `json:"Id"`
	Tags      []string `json:",omitempty"`
	Created   int64
	CreatedBy string `json:",omitempty"`
	Size      int64
}

type APIImages struct {
	Repository  string `json:",omitempty"`
	Tag         string `json:",omitempty"`
	ID          string `json:"Id"`
	Created     int64
	Size        int64
	VirtualSize int64
}

type APIInfo struct {
	Debug              bool
	Containers         int
	Images             int
	NFd                int    `json:",omitempty"`
	NGoroutines        int    `json:",omitempty"`
	MemoryLimit        bool   `json:",omitempty"`
	SwapLimit          bool   `json:",omitempty"`
	IPv4Forwarding     bool   `json:",omitempty"`
	LXCVersion         string `json:",omitempty"`
	NEventsListener    int    `json:",omitempty"`
	KernelVersion      string `json:",omitempty"`
	IndexServerAddress string `json:",omitempty"`
}

type APITop struct {
	Titles    []string
	Processes [][]string
}

type APIRmi struct {
	Deleted  string `json:",omitempty"`
	Untagged string `json:",omitempty"`
}

type APIContainers struct {
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

func (self *APIContainers) ToLegacy() APIContainersOld {
	return APIContainersOld{
		ID:         self.ID,
		Image:      self.Image,
		Command:    self.Command,
		Created:    self.Created,
		Status:     self.Status,
		Ports:      displayablePorts(self.Ports),
		SizeRw:     self.SizeRw,
		SizeRootFs: self.SizeRootFs,
	}
}

type APIContainersOld struct {
	ID         string `json:"Id"`
	Image      string
	Command    string
	Created    int64
	Status     string
	Ports      string
	SizeRw     int64
	SizeRootFs int64
}

type APIID struct {
	ID string `json:"Id"`
}

type APIRun struct {
	ID       string   `json:"Id"`
	Warnings []string `json:",omitempty"`
}

type APIPort struct {
	PrivatePort int64
	PublicPort  int64
	Type        string
	IP          string
}

type APIVersion struct {
	Version   string
	GitCommit string `json:",omitempty"`
	GoVersion string `json:",omitempty"`
}

type APIWait struct {
	StatusCode int
}

type APIAuth struct {
	Status string
}

type APIImageConfig struct {
	ID string `json:"Id"`
	*Config
}

type APICopy struct {
	Resource string
	HostPath string
}
