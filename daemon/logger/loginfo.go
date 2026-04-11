package logger

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/moby/moby/v2/daemon/internal/stringid"
)

// Info provides enough information for a logging driver to do its function.
type Info struct {
	Config              map[string]string
	ContainerID         string
	ContainerName       string
	ContainerEntrypoint string
	ContainerArgs       []string
	ContainerImageID    string
	ContainerImageName  string
	ContainerCreated    time.Time
	ContainerEnv        []string
	ContainerLabels     map[string]string
	LogPath             string
	DaemonName          string
}

// ExtraAttributes returns the user-defined extra attributes (labels,
// environment variables) in key-value format. This can be used by log drivers
// that support metadata to add more context to a log.
func (info *Info) ExtraAttributes(keyMod func(string) string) (map[string]string, error) {
	extra := make(map[string]string)

	if labels, ok := info.Config["labels"]; ok && labels != "" {
		for l := range strings.SplitSeq(labels, ",") {
			if v, ok := info.ContainerLabels[l]; ok {
				if keyMod != nil {
					l = keyMod(l)
				}
				extra[l] = v
			}
		}
	}

	if labelsRegex, ok := info.Config["labels-regex"]; ok && labelsRegex != "" {
		re, err := regexp.Compile(labelsRegex)
		if err != nil {
			return nil, err
		}
		for k, v := range info.ContainerLabels {
			if re.MatchString(k) {
				if keyMod != nil {
					k = keyMod(k)
				}
				extra[k] = v
			}
		}
	}

	envMapping := make(map[string]string)
	for _, e := range info.ContainerEnv {
		if k, v, ok := strings.Cut(e, "="); ok {
			envMapping[k] = v
		}
	}

	// Code below is only to handle adding attributes based on env-vars.
	if len(envMapping) == 0 {
		return extra, nil
	}

	if env, ok := info.Config["env"]; ok && env != "" {
		for l := range strings.SplitSeq(env, ",") {
			if v, ok := envMapping[l]; ok {
				if keyMod != nil {
					l = keyMod(l)
				}
				extra[l] = v
			}
		}
	}

	if envRegex, ok := info.Config["env-regex"]; ok && envRegex != "" {
		re, err := regexp.Compile(envRegex)
		if err != nil {
			return nil, err
		}
		for k, v := range envMapping {
			if re.MatchString(k) {
				if keyMod != nil {
					k = keyMod(k)
				}
				extra[k] = v
			}
		}
	}

	return extra, nil
}

// Hostname returns the hostname from the underlying OS.
func (info *Info) Hostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("logger: can not resolve hostname: %v", err)
	}
	return hostname, nil
}

// Command returns the command that the container being logged was
// started with. The Entrypoint is prepended to the container
// arguments.
func (info *Info) Command() string {
	terms := []string{info.ContainerEntrypoint}
	terms = append(terms, info.ContainerArgs...)
	command := strings.Join(terms, " ")
	return command
}

// ID returns the container ID-prefix (truncated ID).
func (info *Info) ID() string {
	return stringid.TruncateID(info.ContainerID)
}

// FullID returns the container ID.
func (info *Info) FullID() string {
	return info.ContainerID
}

// Name returns the container name.
func (info *Info) Name() string {
	return strings.TrimPrefix(info.ContainerName, "/")
}

// ImageID returns the ID-prefix (truncated ID) of the image the container was created from.
func (info *Info) ImageID() string {
	return stringid.TruncateID(info.ContainerImageID)
}

// ImageFullID returns the ID (digest) of the image the container was created from.
func (info *Info) ImageFullID() string {
	return info.ContainerImageID
}

// ImageName returns the name of the image the container was created from.
func (info *Info) ImageName() string {
	return info.ContainerImageName
}
