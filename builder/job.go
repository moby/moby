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

	"github.com/docker/docker/api"
	"github.com/docker/docker/builder/parser"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
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

func (b *BuilderJob) Install() {
	b.Engine.Register("build", b.CmdBuild)
	b.Engine.Register("build_config", b.CmdBuildConfig)
}

func (b *BuilderJob) CmdBuild(job *engine.Job) error {
	if len(job.Args) != 0 {
		return fmt.Errorf("Usage: %s\n", job.Name)
	}
	var (
		dockerfileName = job.Getenv("dockerfile")
		remoteURL      = job.Getenv("remote")
		repoName       = job.Getenv("t")
		suppressOutput = job.GetenvBool("q")
		noCache        = job.GetenvBool("nocache")
		rm             = job.GetenvBool("rm")
		forceRm        = job.GetenvBool("forcerm")
		pull           = job.GetenvBool("pull")
		memory         = job.GetenvInt64("memory")
		memorySwap     = job.GetenvInt64("memswap")
		cpuShares      = job.GetenvInt64("cpushares")
		cpuSetCpus     = job.Getenv("cpusetcpus")
		authConfig     = &registry.AuthConfig{}
		configFile     = &registry.ConfigFile{}
		tag            string
		context        io.ReadCloser
	)

	job.GetenvJson("authConfig", authConfig)
	job.GetenvJson("configFile", configFile)

	repoName, tag = parsers.ParseRepositoryTag(repoName)
	if repoName != "" {
		if err := registry.ValidateRepositoryName(repoName); err != nil {
			return err
		}
		if len(tag) > 0 {
			if err := graph.ValidateTagName(tag); err != nil {
				return err
			}
		}
	}

	if remoteURL == "" {
		context = ioutil.NopCloser(job.Stdin)
	} else if urlutil.IsGitURL(remoteURL) {
		if !urlutil.IsGitTransport(remoteURL) {
			remoteURL = "https://" + remoteURL
		}
		root, err := ioutil.TempDir("", "docker-build-git")
		if err != nil {
			return err
		}
		defer os.RemoveAll(root)

		if output, err := exec.Command("git", "clone", "--recursive", remoteURL, root).CombinedOutput(); err != nil {
			return fmt.Errorf("Error trying to use git: %s (%s)", err, output)
		}

		c, err := archive.Tar(root, archive.Uncompressed)
		if err != nil {
			return err
		}
		context = c
	} else if urlutil.IsURL(remoteURL) {
		f, err := utils.Download(remoteURL)
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
		dockerfileName = api.DefaultDockerfileName

		c, err := archive.Generate(dockerfileName, string(dockerFile))
		if err != nil {
			return err
		}
		context = c
	}
	defer context.Close()

	sf := utils.NewStreamFormatter(job.GetenvBool("json"))

	builder := &Builder{
		Daemon: b.Daemon,
		Engine: b.Engine,
		OutStream: &utils.StdoutFormater{
			Writer:          job.Stdout,
			StreamFormatter: sf,
		},
		ErrStream: &utils.StderrFormater{
			Writer:          job.Stdout,
			StreamFormatter: sf,
		},
		Verbose:         !suppressOutput,
		UtilizeCache:    !noCache,
		Remove:          rm,
		ForceRemove:     forceRm,
		Pull:            pull,
		OutOld:          job.Stdout,
		StreamFormatter: sf,
		AuthConfig:      authConfig,
		AuthConfigFile:  configFile,
		dockerfileName:  dockerfileName,
		cpuShares:       cpuShares,
		cpuSetCpus:      cpuSetCpus,
		memory:          memory,
		memorySwap:      memorySwap,
		cancelled:       job.WaitCancelled(),
	}

	id, err := builder.Run(context)
	if err != nil {
		return err
	}

	if repoName != "" {
		b.Daemon.Repositories().Set(repoName, tag, id, true)
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
