package docker

type ApiHistory struct {
	Id        string
	Created   int64
	CreatedBy string `json:",omitempty"`
}

type ApiImages struct {
	Repository string `json:",omitempty"`
	Tag        string `json:",omitempty"`
	Id         string
	Created    int64
}

type ApiInfo struct {
	Debug       bool
	Containers  int
	Images      int
	NFd         int  `json:",omitempty"`
	NGoroutines int  `json:",omitempty"`
	MemoryLimit bool `json:",omitempty"`
	SwapLimit   bool `json:",omitempty"`
}

type ApiContainers struct {
	Id      string
	Image   string
	Command string
	Created int64
	Status  string
	Ports   string
}

type ApiSearch struct {
	Name        string
	Description string
}

type ApiId struct {
	Id string
}

type ApiRun struct {
	Id       string
	Warnings []string `json:",omitempty"`
}

type ApiPort struct {
	Port string
}

type ApiVersion struct {
	Version   string
	GitCommit string `json:",omitempty"`
	GoVersion string `json:",omitempty"`
}

type ApiWait struct {
	StatusCode int
}

type ApiAuth struct {
	Status string
}

type ApiImageConfig struct {
	Id string
	*Config
}
