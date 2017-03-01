package main

import (
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

type Moby struct {
	Kernel   string
	Init     string
	System   []MobyImage
	Database []struct {
		File  string
		Value string
	}
}

type MobyImage struct {
	Name         string
	Image        string
	Capabilities []string
	Binds        []string
	OomScoreAdj  int64 `yaml:"oom_score_adj"`
	Command      []string
	NetworkMode  string `yaml:"network_mode"`
}

const riddler = "mobylinux/riddler:7d4545d8b8ac2700971a83f12a3446a76db28c14@sha256:11b7310df6482fc38aa52b419c2ef1065d7b9207c633d47554e13aa99f6c0b72"

func NewConfig(config []byte) (*Moby, error) {
	m := Moby{}

	err := yaml.Unmarshal(config, &m)
	if err != nil {
		return &m, err
	}

	return &m, nil
}

func ConfigToRun(image *MobyImage) []string {
	// riddler arguments
	args := []string{"run", "--rm", "-v", "/var/run/docker.sock:/var/run/docker.sock", riddler, image.Image, "/containers/" + image.Name}
	// docker arguments
	args = append(args, "--cap-drop", "all")
	for _, cap := range image.Capabilities {
		if strings.ToUpper(cap)[0:4] == "CAP_" {
			cap = cap[4:]
		}
		args = append(args, "--cap-add", cap)
	}
	if image.OomScoreAdj != 0 {
		args = append(args, "--oom-score-adj", strconv.FormatInt(image.OomScoreAdj, 10))
	}
	if image.NetworkMode != "" {
		args = append(args, "--net", image.NetworkMode)
	}
	for _, bind := range image.Binds {
		args = append(args, "-v", bind)
	}
	// image
	args = append(args, image.Image)
	// command
	args = append(args, image.Command...)

	return args
}
