package docker

type SimpleMessage struct {
	Message string
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

type VersionOut struct {
	Version             string
	GitCommit           string
	MemoryLimitDisabled bool
}
