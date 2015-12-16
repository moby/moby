package client

import (
	"runtime"
	"text/template"
	"time"

	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/dockerversion"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/docker/utils"
)

var versionTemplate = `Client:
 Version:      {{.Client.Version}}
 API version:  {{.Client.APIVersion}}
 Go version:   {{.Client.GoVersion}}
 Git commit:   {{.Client.GitCommit}}
 Built:        {{.Client.BuildTime}}
 OS/Arch:      {{.Client.Os}}/{{.Client.Arch}}{{if .Client.Experimental}}
 Experimental: {{.Client.Experimental}}{{end}}{{if .ServerOK}}

Server:
 Version:      {{.Server.Version}}
 API version:  {{.Server.APIVersion}}
 Go version:   {{.Server.GoVersion}}
 Git commit:   {{.Server.GitCommit}}
 Built:        {{.Server.BuildTime}}
 OS/Arch:      {{.Server.Os}}/{{.Server.Arch}}{{if .Server.Experimental}}
 Experimental: {{.Server.Experimental}}{{end}}{{end}}`

// CmdVersion shows Docker version information.
//
// Available version information is shown for: client Docker version, client API version, client Go version, client Git commit, client OS/Arch, server Docker version, server API version, server Go version, server Git commit, and server OS/Arch.
//
// Usage: docker version
func (cli *DockerCli) CmdVersion(args ...string) (err error) {
	cmd := Cli.Subcmd("version", nil, Cli.DockerCommands["version"].Description, true)
	tmplStr := cmd.String([]string{"f", "#format", "-format"}, "", "Format the output using the given go template")
	cmd.Require(flag.Exact, 0)

	cmd.ParseFlags(args, true)

	templateFormat := versionTemplate
	if *tmplStr != "" {
		templateFormat = *tmplStr
	}

	var tmpl *template.Template
	if tmpl, err = template.New("").Funcs(funcMap).Parse(templateFormat); err != nil {
		return Cli.StatusError{StatusCode: 64,
			Status: "Template parsing error: " + err.Error()}
	}

	vd := types.VersionResponse{
		Client: &types.Version{
			Version:      dockerversion.Version,
			APIVersion:   version.Version(cli.client.ClientVersion()),
			GoVersion:    runtime.Version(),
			GitCommit:    dockerversion.GitCommit,
			BuildTime:    dockerversion.BuildTime,
			Os:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			Experimental: utils.ExperimentalBuild(),
		},
	}

	serverVersion, err := cli.client.ServerVersion()
	if err == nil {
		vd.Server = &serverVersion
	}

	// first we need to make BuildTime more human friendly
	t, errTime := time.Parse(time.RFC3339Nano, vd.Client.BuildTime)
	if errTime == nil {
		vd.Client.BuildTime = t.Format(time.ANSIC)
	}

	if vd.ServerOK() {
		t, errTime = time.Parse(time.RFC3339Nano, vd.Server.BuildTime)
		if errTime == nil {
			vd.Server.BuildTime = t.Format(time.ANSIC)
		}
	}

	if err2 := tmpl.Execute(cli.out, vd); err2 != nil && err == nil {
		err = err2
	}
	cli.out.Write([]byte{'\n'})
	return err
}
