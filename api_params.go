package docker

type ApiHistory struct {
	Id        string
	Created   int64
	CreatedBy string
}

type ApiImages struct {
	Repository string `json:",omitempty"`
	Tag        string `json:",omitempty"`
	Id         string
	Created    int64 `json:",omitempty"`
	Size       int64
	ParentSize int64
}

type ApiInfo struct {
	Containers  int
	Version     string
	Images      int
	Debug       bool
	GoVersion   string
	NFd         int `json:",omitempty"`
	NGoroutines int `json:",omitempty"`
}

type ApiContainers struct {
	Id         string
	Image      string `json:",omitempty"`
	Command    string `json:",omitempty"`
	Created    int64  `json:",omitempty"`
	Status     string `json:",omitempty"`
	Ports      string `json:",omitempty"`
	SizeRw     int64
	SizeRootFs int64
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
	Warnings []string
}

type ApiPort struct {
	Port string
}

type ApiVersion struct {
	Version     string
	GitCommit   string
	MemoryLimit bool
	SwapLimit   bool
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
