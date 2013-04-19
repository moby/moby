package docker

type SimpleMessage struct {
	Message string
}

type HistoryIn struct {
	Name string
}

type HistoryOut struct {
	Id        string
	Created   string
	CreatedBy string
}

type ImagesIn struct {
	NameFilter string
	Quiet      bool
	All        bool
}

type ImagesOut struct {
	Repository string `json:",omitempty"`
	Tag        string `json:",omitempty"`
	Id         string
	Created    string `json:",omitempty"`
}

type InfoOut struct {
	Containers  int
	Version     string
	Images      int
	Debug       bool
	NFd         int `json:",omitempty"`
	NGoroutines int `json:",omitempty"`
}

type PsIn struct {
	Quiet bool
	All   bool
	Full  bool
	Last  int
}

type PsOut struct {
	Id      string
	Image   string `json:",omitempty"`
	Command string `json:",omitempty"`
	Created string `json:",omitempty"`
	Status  string `json:",omitempty"`
}

type LogsIn struct {
	Name string
}

type LogsOut struct {
	Stdout string
	Stderr string
}

type VersionOut struct {
	Version             string
	GitCommit           string
	MemoryLimitDisabled bool
}
