package imgstore

import (
	"time"
)

type Image struct {
	Schema SchemaVersion

	// Each image has exactly one name.
	// Images are self-describing: their manifest includes their name.
	//
	// An image name is made of 2 parts:
	// 	- A repository, for example 'shykes/myapp'.
	//	- A tag, for example 'v2'.
	// All image names are part of a globally unique namespace, which is
	// enforced by a common federated naming system. This allows using
	// repositories as a trust unit: if you know how much you trust the
	// entity behind a certain repository name, you can reliably enforce
	// that level of trust for all images under that name, because Docker
	// guarantees that they were published by the same entity.
	Name struct {
		Repository string
		Tag        string
	}

	Author      string
	Created     time.Time
	Description string

	// A map of commands exposed by the container by default.
	// By convention the default command should be called 'main'.
	// 'docker start' or 'docker run', when not given any arguments, will start that command.
	Commands map[string]struct {
		// The command to execute
		Path string
		Args string

		// Which hardware architectures and OS can this command run on?
		Arch []Arch
		OS   []OS

		// The user id under which to run this command
		User      string
		Env       []string
		Tty       bool
		OpenStdin bool
	}

	FS []struct {
		Layer LayerHash
		Op    LayerOp
		Dst   string
	}

	Triggers map[TriggerType][]string
}

type SchemaVersion uint32

const (
	Schema_1_1 SchemaVersion = iota + 1
)

type Arch string

const (
	X86    Arch = "x86"
	X86_64 Arch = "x86_64"
	Arm    Arch = "arm"
)

type OS string

const (
	Linux   OS = "linux"
	Darwin  OS = "darwin"
	Windows OS = "windows"
)

type TriggerType string

const (
	OnBuild TriggerType = "onbuild"
)

type LayerHash struct {
	Type  HashType
	Value string
}

type HashType string

const (
	Tarsum1 HashType = "tarsum1"
)

type LayerOp string

const (
	OpUnpack  LayerOp = "unpack"
	OpCopy    LayerOp = "copy"
	OpMount   LayerOp = "mount"
	OpMountRO LayerOp = "mountro"
)
