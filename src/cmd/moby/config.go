package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

// Moby is the type of a Moby config file
type Moby struct {
	Kernel struct {
		Image   string
		Cmdline string
	}
	Init   string
	System []MobyImage
	Daemon []MobyImage
	Files  []struct {
		Path     string
		Contents string
	}
	Outputs []struct {
		Format  string
		Project string
		Bucket  string
		Family  string
		Public  bool
		Replace bool
	}
}

// MobyImage is the type of an image config, based on Compose
type MobyImage struct {
	Name         string
	Image        string
	Capabilities []string
	Binds        []string
	OomScoreAdj  int64 `yaml:"oom_score_adj"`
	Command      []string
	NetworkMode  string `yaml:"network_mode"`
	Pid          string
	Ipc          string
	Uts          string
	ReadOnly     bool `yaml:"read_only"`
}

const riddler = "mobylinux/riddler:2b4051422b155f659019f9e3fef8cca04e153f5c@sha256:f4bb0c39f1e5c636ed52ebd3ed8ec447ca6c0dc554ffb5784cbeff423ac70d34"

// NewConfig parses a config file
func NewConfig(config []byte) (*Moby, error) {
	m := Moby{}

	err := yaml.Unmarshal(config, &m)
	if err != nil {
		return &m, err
	}

	return &m, nil
}

// ConfigToOCI converts a config specification to an OCI config file
func ConfigToOCI(image *MobyImage) (string, error) {
	// riddler arguments
	args := []string{"-v", "/var/run/docker.sock:/var/run/docker.sock", riddler, image.Image}
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
		// TODO only "host" supported
		args = append(args, "--net="+image.NetworkMode)
	}
	if image.Pid != "" {
		// TODO only "host" supported
		args = append(args, "--pid="+image.Pid)
	}
	if image.Ipc != "" {
		// TODO only "host" supported
		args = append(args, "--ipc="+image.Ipc)
	}
	if image.Uts != "" {
		// TODO only "host" supported
		args = append(args, "--uts="+image.Uts)
	}
	for _, bind := range image.Binds {
		args = append(args, "-v", bind)
	}
	if image.ReadOnly {
		args = append(args, "--read-only")
	}
	// image
	args = append(args, image.Image)
	// command
	args = append(args, image.Command...)

	config, err := dockerRun(args...)
	if err != nil {
		return "", fmt.Errorf("Failed to run riddler to get config.json: %v", err)
	}

	return string(config), nil
}

func filesystem(m *Moby) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	for _, f := range m.Files {
		if f.Path == "" {
			return buf, errors.New("Did not specify path for file")
		}
		if f.Contents == "" {
			return buf, errors.New("Contents of file not specified")
		}
		// we need all the leading directories
		parts := strings.Split(path.Dir(f.Path), "/")
		root := ""
		for _, p := range parts {
			if p == "." || p == "/" {
				continue
			}
			if root == "" {
				root = p
			} else {
				root = root + "/" + p
			}
			hdr := &tar.Header{
				Name:     root,
				Typeflag: tar.TypeDir,
				Mode:     0700,
			}
			err := tw.WriteHeader(hdr)
			if err != nil {
				return buf, err
			}
		}
		hdr := &tar.Header{
			Name: f.Path,
			Mode: 0600,
			Size: int64(len(f.Contents)),
		}
		err := tw.WriteHeader(hdr)
		if err != nil {
			return buf, err
		}
		_, err = tw.Write([]byte(f.Contents))
		if err != nil {
			return buf, err
		}
	}
	return buf, nil
}
