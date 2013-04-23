package docker

type ApiHistory struct {
	Id        string
	Created   string
	CreatedBy string
}

type ApiImages struct {
	Repository string `json:",omitempty"`
	Tag        string `json:",omitempty"`
	Id         string
	Created    string `json:",omitempty"`
}

type ApiInfo struct {
	Containers  int
	Version     string
	Images      int
	Debug       bool
	NFd         int `json:",omitempty"`
	NGoroutines int `json:",omitempty"`
}

type ApiContainers struct {
	Id      string
	Image   string `json:",omitempty"`
	Command string `json:",omitempty"`
	Created string `json:",omitempty"`
	Status  string `json:",omitempty"`
}

type ApiLogs struct {
	Stdout string
	Stderr string
}

type ApiPort struct {
	Port string
}

type ApiVersion struct {
	Version             string
	GitCommit           string
	MemoryLimitDisabled bool
}
