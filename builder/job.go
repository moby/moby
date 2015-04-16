package builder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/docker/docker/api"
	"github.com/docker/docker/builder/parser"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
)

// whitelist of commands allowed for a commit/import
var validCommitCommands = map[string]bool{
	"entrypoint": true,
	"cmd":        true,
	"user":       true,
	"workdir":    true,
	"env":        true,
	"volume":     true,
	"expose":     true,
	"onbuild":    true,
}

type BuilderJob struct {
	Engine *engine.Engine
	Daemon *daemon.Daemon
}

type Config struct {
	DockerfileName string
	RemoteURL      string
	RepoName       string
	SuppressOutput bool
	NoCache        bool
	Remove         bool
	ForceRemove    bool
	Pull           bool
	JSONFormat     bool
	Memory         int64
	MemorySwap     int64
	CpuShares      int64
	CpuSetCpus     string
	CpuSetMems     string
	AuthConfig     *registry.AuthConfig
	ConfigFile     *registry.ConfigFile

	Stdout *engine.Output
	Stderr *engine.Output
	Stdin  *engine.Input
	// When closed, the job has been cancelled.
	// Note: not all jobs implement cancellation.
	// See Job.Cancel() and Job.WaitCancelled()
	cancelled  chan struct{}
	cancelOnce sync.Once
}

// When called, causes the Job.WaitCancelled channel to unblock.
func (b *Config) Cancel() {
	b.cancelOnce.Do(func() {
		close(b.cancelled)
	})
}

// Returns a channel which is closed ("never blocks") when the job is cancelled.
func (b *Config) WaitCancelled() <-chan struct{} {
	return b.cancelled
}

func NewBuildConfig(logging bool, err io.Writer) *Config {
	c := &Config{
		Stdout:    engine.NewOutput(),
		Stderr:    engine.NewOutput(),
		Stdin:     engine.NewInput(),
		cancelled: make(chan struct{}),
	}
	if logging {
		c.Stderr.Add(ioutils.NopWriteCloser(err))
	}
	return c
}

func (b *BuilderJob) Install() {
	b.Engine.Register("build_config", b.CmdBuildConfig)
}

func (b *BuilderJob) CmdBuild(buildConfig *Config) error {
	var (
		repoName string
		tag      string
		context  io.ReadCloser
	)

	repoName, tag = parsers.ParseRepositoryTag(buildConfig.RepoName)
	if repoName != "" {
		if err := registry.ValidateRepositoryName(buildConfig.RepoName); err != nil {
			return err
		}
		if len(tag) > 0 {
			if err := graph.ValidateTagName(tag); err != nil {
				return err
			}
		}
	}

	if buildConfig.RemoteURL == "" {
		context = ioutil.NopCloser(buildConfig.Stdin)
	} else if urlutil.IsGitURL(buildConfig.RemoteURL) {
		if !urlutil.IsGitTransport(buildConfig.RemoteURL) {
			buildConfig.RemoteURL = "https://" + buildConfig.RemoteURL
		}
		root, err := ioutil.TempDir("", "docker-build-git")
		if err != nil {
			return err
		}
		defer os.RemoveAll(root)

		if output, err := exec.Command("git", "clone", "--recursive", buildConfig.RemoteURL, root).CombinedOutput(); err != nil {
			return fmt.Errorf("Error trying to use git: %s (%s)", err, output)
		}

		c, err := archive.Tar(root, archive.Uncompressed)
		if err != nil {
			return err
		}
		context = c
	} else if urlutil.IsURL(buildConfig.RemoteURL) {
		f, err := httputils.Download(buildConfig.RemoteURL)
		if err != nil {
			return err
		}
		defer f.Body.Close()
		dockerFile, err := ioutil.ReadAll(f.Body)
		if err != nil {
			return err
		}

		// When we're downloading just a Dockerfile put it in
		// the default name - don't allow the client to move/specify it
		buildConfig.DockerfileName = api.DefaultDockerfileName

		c, err := archive.Generate(buildConfig.DockerfileName, string(dockerFile))
		if err != nil {
			return err
		}
		context = c
	}
	defer context.Close()

	sf := streamformatter.NewStreamFormatter(buildConfig.JSONFormat)

	builder := &Builder{
		Daemon: b.Daemon,
		Engine: b.Engine,
		OutStream: &streamformatter.StdoutFormater{
			Writer:          buildConfig.Stdout,
			StreamFormatter: sf,
		},
		ErrStream: &streamformatter.StderrFormater{
			Writer:          buildConfig.Stdout,
			StreamFormatter: sf,
		},
		Verbose:         !buildConfig.SuppressOutput,
		UtilizeCache:    !buildConfig.NoCache,
		Remove:          buildConfig.Remove,
		ForceRemove:     buildConfig.ForceRemove,
		Pull:            buildConfig.Pull,
		OutOld:          buildConfig.Stdout,
		StreamFormatter: sf,
		AuthConfig:      buildConfig.AuthConfig,
		ConfigFile:      buildConfig.ConfigFile,
		dockerfileName:  buildConfig.DockerfileName,
		cpuShares:       buildConfig.CpuShares,
		cpuSetCpus:      buildConfig.CpuSetCpus,
		cpuSetMems:      buildConfig.CpuSetMems,
		memory:          buildConfig.Memory,
		memorySwap:      buildConfig.MemorySwap,
		cancelled:       buildConfig.WaitCancelled(),
	}

	id, err := builder.Run(context)
	if err != nil {
		return err
	}

	if repoName != "" {
		b.Daemon.Repositories().Tag(repoName, tag, id, true)
	}
	return nil
}

func (b *BuilderJob) CmdBuildConfig(job *engine.Job) error {
	if len(job.Args) != 0 {
		return fmt.Errorf("Usage: %s\n", job.Name)
	}

	var (
		changes   = job.GetenvList("changes")
		newConfig runconfig.Config
	)

	if err := job.GetenvJson("config", &newConfig); err != nil {
		return err
	}

	ast, err := parser.Parse(bytes.NewBufferString(strings.Join(changes, "\n")))
	if err != nil {
		return err
	}

	// ensure that the commands are valid
	for _, n := range ast.Children {
		if !validCommitCommands[n.Value] {
			return fmt.Errorf("%s is not a valid change command", n.Value)
		}
	}

	builder := &Builder{
		Daemon:        b.Daemon,
		Engine:        b.Engine,
		Config:        &newConfig,
		OutStream:     ioutil.Discard,
		ErrStream:     ioutil.Discard,
		disableCommit: true,
	}

	for i, n := range ast.Children {
		if err := builder.dispatch(i, n); err != nil {
			return err
		}
	}

	if err := json.NewEncoder(job.Stdout).Encode(builder.Config); err != nil {
		return err
	}
	return nil
}
