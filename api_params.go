package docker

import "strings"

type (
	APIHistory struct {
		ID        string   `json:"Id"`
		Tags      []string `json:",omitempty"`
		Created   int64
		CreatedBy string `json:",omitempty"`
		Size      int64
	}

	APIImages struct {
		ID          string   `json:"Id"`
		RepoTags    []string `json:",omitempty"`
		Created     int64
		Size        int64
		VirtualSize int64
		ParentId    string `json:",omitempty"`
	}

	APIImagesOld struct {
		Repository  string `json:",omitempty"`
		Tag         string `json:",omitempty"`
		ID          string `json:"Id"`
		Created     int64
		Size        int64
		VirtualSize int64
	}

	APIInfo struct {
		Debug              bool
		Containers         int
		Images             int
		Driver             string      `json:",omitempty"`
		DriverStatus       [][2]string `json:",omitempty"`
		NFd                int         `json:",omitempty"`
		NGoroutines        int         `json:",omitempty"`
		MemoryLimit        bool        `json:",omitempty"`
		SwapLimit          bool        `json:",omitempty"`
		IPv4Forwarding     bool        `json:",omitempty"`
		LXCVersion         string      `json:",omitempty"`
		NEventsListener    int         `json:",omitempty"`
		KernelVersion      string      `json:",omitempty"`
		IndexServerAddress string      `json:",omitempty"`
	}

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
	APIContainer struct {
		*Container
		HostConfig *HostConfig
	}
)

func (api APIImages) ToLegacy() []APIImagesOld {
	outs := []APIImagesOld{}
	for _, repotag := range api.RepoTags {
		components := strings.SplitN(repotag, ":", 2)
		outs = append(outs, APIImagesOld{
			ID:          api.ID,
			Repository:  components[0],
			Tag:         components[1],
			Created:     api.Created,
			Size:        api.Size,
			VirtualSize: api.VirtualSize,
		})
	}
	return outs
}

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
