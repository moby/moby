package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"text/template"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/common"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/networkfs/resolvconf"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/timeutils"
	"github.com/docker/docker/pkg/units"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

const (
	tarHeaderSize = 512
)

func (cli *DockerCli) CmdUnpause(args ...string) error {
	cmd := cli.Subcmd("unpause", "CONTAINER [CONTAINER...]", "Unpause all processes within a container", true)
	cmd.Require(flag.Min, 1)
	utils.ParseFlags(cmd, args, false)

	var encounteredError error
	for _, name := range cmd.Args() {
		if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/unpause", name), nil, false)); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to unpause container named %s", name)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}

func (cli *DockerCli) CmdPause(args ...string) error {
	cmd := cli.Subcmd("pause", "CONTAINER [CONTAINER...]", "Pause all processes within a container", true)
	cmd.Require(flag.Min, 1)
	utils.ParseFlags(cmd, args, false)

	var encounteredError error
	for _, name := range cmd.Args() {
		if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/pause", name), nil, false)); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to pause container named %s", name)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}

func (cli *DockerCli) CmdRename(args ...string) error {
	cmd := cli.Subcmd("rename", "OLD_NAME NEW_NAME", "Rename a container", true)
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 2 {
		cmd.Usage()
		return nil
	}
	old_name := cmd.Arg(0)
	new_name := cmd.Arg(1)

	if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/rename?name=%s", old_name, new_name), nil, false)); err != nil {
		fmt.Fprintf(cli.err, "%s\n", err)
		return fmt.Errorf("Error: failed to rename container named %s", old_name)
	}
	return nil
}

func (cli *DockerCli) CmdInspect(args ...string) error {
	cmd := cli.Subcmd("inspect", "CONTAINER|IMAGE [CONTAINER|IMAGE...]", "Return low-level information on a container or image", true)
	tmplStr := cmd.String([]string{"f", "#format", "-format"}, "", "Format the output using the given go template")
	cmd.Require(flag.Min, 1)

	utils.ParseFlags(cmd, args, true)

	var tmpl *template.Template
	if *tmplStr != "" {
		var err error
		if tmpl, err = template.New("").Funcs(funcMap).Parse(*tmplStr); err != nil {
			fmt.Fprintf(cli.err, "Template parsing error: %v\n", err)
			return &utils.StatusError{StatusCode: 64,
				Status: "Template parsing error: " + err.Error()}
		}
	}

	indented := new(bytes.Buffer)
	indented.WriteByte('[')
	status := 0

	for _, name := range cmd.Args() {
		obj, _, err := readBody(cli.call("GET", "/containers/"+name+"/json", nil, false))
		if err != nil {
			if strings.Contains(err.Error(), "Too many") {
				fmt.Fprintf(cli.err, "Error: %v", err)
				status = 1
				continue
			}

			obj, _, err = readBody(cli.call("GET", "/images/"+name+"/json", nil, false))
			if err != nil {
				if strings.Contains(err.Error(), "No such") {
					fmt.Fprintf(cli.err, "Error: No such image or container: %s\n", name)
				} else {
					fmt.Fprintf(cli.err, "%s", err)
				}
				status = 1
				continue
			}
		}

		if tmpl == nil {
			if err = json.Indent(indented, obj, "", "    "); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				status = 1
				continue
			}
		} else {
			// Has template, will render
			var value interface{}
			if err := json.Unmarshal(obj, &value); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				status = 1
				continue
			}
			if err := tmpl.Execute(cli.out, value); err != nil {
				return err
			}
			cli.out.Write([]byte{'\n'})
		}
		indented.WriteString(",")
	}

	if indented.Len() > 1 {
		// Remove trailing ','
		indented.Truncate(indented.Len() - 1)
	}
	indented.WriteString("]\n")

	if tmpl == nil {
		if _, err := io.Copy(cli.out, indented); err != nil {
			return err
		}
	}

	if status != 0 {
		return &utils.StatusError{StatusCode: status}
	}
	return nil
}

func (cli *DockerCli) CmdTop(args ...string) error {
	cmd := cli.Subcmd("top", "CONTAINER [ps OPTIONS]", "Display the running processes of a container", true)
	cmd.Require(flag.Min, 1)

	utils.ParseFlags(cmd, args, true)

	val := url.Values{}
	if cmd.NArg() > 1 {
		val.Set("ps_args", strings.Join(cmd.Args()[1:], " "))
	}

	stream, _, err := cli.call("GET", "/containers/"+cmd.Arg(0)+"/top?"+val.Encode(), nil, false)
	if err != nil {
		return err
	}
	var procs engine.Env
	if err := procs.Decode(stream); err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, strings.Join(procs.GetList("Titles"), "\t"))
	processes := [][]string{}
	if err := procs.GetJson("Processes", &processes); err != nil {
		return err
	}
	for _, proc := range processes {
		fmt.Fprintln(w, strings.Join(proc, "\t"))
	}
	w.Flush()
	return nil
}

func (cli *DockerCli) CmdPort(args ...string) error {
	cmd := cli.Subcmd("port", "CONTAINER [PRIVATE_PORT[/PROTO]]", "List port mappings for the CONTAINER, or lookup the public-facing port that\nis NAT-ed to the PRIVATE_PORT", true)
	cmd.Require(flag.Min, 1)
	utils.ParseFlags(cmd, args, true)

	stream, _, err := cli.call("GET", "/containers/"+cmd.Arg(0)+"/json", nil, false)
	if err != nil {
		return err
	}

	env := engine.Env{}
	if err := env.Decode(stream); err != nil {
		return err
	}
	ports := nat.PortMap{}
	if err := env.GetSubEnv("NetworkSettings").GetJson("Ports", &ports); err != nil {
		return err
	}

	if cmd.NArg() == 2 {
		var (
			port  = cmd.Arg(1)
			proto = "tcp"
			parts = strings.SplitN(port, "/", 2)
		)

		if len(parts) == 2 && len(parts[1]) != 0 {
			port = parts[0]
			proto = parts[1]
		}
		natPort := port + "/" + proto
		if frontends, exists := ports[nat.Port(port+"/"+proto)]; exists && frontends != nil {
			for _, frontend := range frontends {
				fmt.Fprintf(cli.out, "%s:%s\n", frontend.HostIp, frontend.HostPort)
			}
			return nil
		}
		return fmt.Errorf("Error: No public port '%s' published for %s", natPort, cmd.Arg(0))
	}

	for from, frontends := range ports {
		for _, frontend := range frontends {
			fmt.Fprintf(cli.out, "%s -> %s:%s\n", from, frontend.HostIp, frontend.HostPort)
		}
	}

	return nil
}

// 'docker rmi IMAGE' removes all images with the name IMAGE
func (cli *DockerCli) CmdRmi(args ...string) error {
	var (
		cmd     = cli.Subcmd("rmi", "IMAGE [IMAGE...]", "Remove one or more images", true)
		force   = cmd.Bool([]string{"f", "-force"}, false, "Force removal of the image")
		noprune = cmd.Bool([]string{"-no-prune"}, false, "Do not delete untagged parents")
	)
	cmd.Require(flag.Min, 1)

	utils.ParseFlags(cmd, args, true)

	v := url.Values{}
	if *force {
		v.Set("force", "1")
	}
	if *noprune {
		v.Set("noprune", "1")
	}

	var encounteredError error
	for _, name := range cmd.Args() {
		body, _, err := readBody(cli.call("DELETE", "/images/"+name+"?"+v.Encode(), nil, false))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to remove one or more images")
		} else {
			outs := engine.NewTable("Created", 0)
			if _, err := outs.ReadListFrom(body); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				encounteredError = fmt.Errorf("Error: failed to remove one or more images")
				continue
			}
			for _, out := range outs.Data {
				if out.Get("Deleted") != "" {
					fmt.Fprintf(cli.out, "Deleted: %s\n", out.Get("Deleted"))
				} else {
					fmt.Fprintf(cli.out, "Untagged: %s\n", out.Get("Untagged"))
				}
			}
		}
	}
	return encounteredError
}

func (cli *DockerCli) CmdHistory(args ...string) error {
	cmd := cli.Subcmd("history", "IMAGE", "Show the history of an image", true)
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only show numeric IDs")
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)

	body, _, err := readBody(cli.call("GET", "/images/"+cmd.Arg(0)+"/history", nil, false))
	if err != nil {
		return err
	}

	outs := engine.NewTable("Created", 0)
	if _, err := outs.ReadListFrom(body); err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprintln(w, "IMAGE\tCREATED\tCREATED BY\tSIZE")
	}

	for _, out := range outs.Data {
		outID := out.Get("Id")
		if !*quiet {
			if *noTrunc {
				fmt.Fprintf(w, "%s\t", outID)
			} else {
				fmt.Fprintf(w, "%s\t", common.TruncateID(outID))
			}

			fmt.Fprintf(w, "%s ago\t", units.HumanDuration(time.Now().UTC().Sub(time.Unix(out.GetInt64("Created"), 0))))

			if *noTrunc {
				fmt.Fprintf(w, "%s\t", out.Get("CreatedBy"))
			} else {
				fmt.Fprintf(w, "%s\t", utils.Trunc(out.Get("CreatedBy"), 45))
			}
			fmt.Fprintf(w, "%s\n", units.HumanSize(float64(out.GetInt64("Size"))))
		} else {
			if *noTrunc {
				fmt.Fprintln(w, outID)
			} else {
				fmt.Fprintln(w, common.TruncateID(outID))
			}
		}
	}
	w.Flush()
	return nil
}

func (cli *DockerCli) CmdRm(args ...string) error {
	cmd := cli.Subcmd("rm", "CONTAINER [CONTAINER...]", "Remove one or more containers", true)
	v := cmd.Bool([]string{"v", "-volumes"}, false, "Remove the volumes associated with the container")
	link := cmd.Bool([]string{"l", "#link", "-link"}, false, "Remove the specified link")
	force := cmd.Bool([]string{"f", "-force"}, false, "Force the removal of a running container (uses SIGKILL)")
	cmd.Require(flag.Min, 1)

	utils.ParseFlags(cmd, args, true)

	val := url.Values{}
	if *v {
		val.Set("v", "1")
	}
	if *link {
		val.Set("link", "1")
	}

	if *force {
		val.Set("force", "1")
	}

	var encounteredError error
	for _, name := range cmd.Args() {
		_, _, err := readBody(cli.call("DELETE", "/containers/"+name+"?"+val.Encode(), nil, false))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to remove one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}

// 'docker kill NAME' kills a running container
func (cli *DockerCli) CmdKill(args ...string) error {
	cmd := cli.Subcmd("kill", "CONTAINER [CONTAINER...]", "Kill a running container using SIGKILL or a specified signal", true)
	signal := cmd.String([]string{"s", "-signal"}, "KILL", "Signal to send to the container")
	cmd.Require(flag.Min, 1)

	utils.ParseFlags(cmd, args, true)

	var encounteredError error
	for _, name := range cmd.Args() {
		if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/kill?signal=%s", name, *signal), nil, false)); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to kill one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}

func (cli *DockerCli) CmdImport(args ...string) error {
	cmd := cli.Subcmd("import", "URL|- [REPOSITORY[:TAG]]", "Create an empty filesystem image and import the contents of the\ntarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz) into it, then\noptionally tag it.", true)
	flChanges := opts.NewListOpts(nil)
	cmd.Var(&flChanges, []string{"c", "-change"}, "Apply Dockerfile instruction to the created image")
	cmd.Require(flag.Min, 1)

	utils.ParseFlags(cmd, args, true)

	var (
		v          = url.Values{}
		src        = cmd.Arg(0)
		repository = cmd.Arg(1)
	)

	v.Set("fromSrc", src)
	v.Set("repo", repository)
	for _, change := range flChanges.GetAll() {
		v.Add("changes", change)
	}
	if cmd.NArg() == 3 {
		fmt.Fprintf(cli.err, "[DEPRECATED] The format 'URL|- [REPOSITORY [TAG]]' has been deprecated. Please use URL|- [REPOSITORY[:TAG]]\n")
		v.Set("tag", cmd.Arg(2))
	}

	if repository != "" {
		//Check if the given image name can be resolved
		repo, _ := parsers.ParseRepositoryTag(repository)
		if err := registry.ValidateRepositoryName(repo); err != nil {
			return err
		}
	}

	var in io.Reader

	if src == "-" {
		in = cli.in
	}

	return cli.stream("POST", "/images/create?"+v.Encode(), in, cli.out, nil)
}

func (cli *DockerCli) CmdPush(args ...string) error {
	cmd := cli.Subcmd("push", "NAME[:TAG]", "Push an image or a repository to the registry", true)
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)

	name := cmd.Arg(0)

	cli.LoadConfigFile()

	remote, tag := parsers.ParseRepositoryTag(name)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(remote)
	if err != nil {
		return err
	}
	// Resolve the Auth config relevant for this server
	authConfig := cli.configFile.ResolveAuthConfig(repoInfo.Index)
	// If we're not using a custom registry, we know the restrictions
	// applied to repository names and can warn the user in advance.
	// Custom repositories can have different rules, and we must also
	// allow pushing by image ID.
	if repoInfo.Official {
		username := authConfig.Username
		if username == "" {
			username = "<user>"
		}
		return fmt.Errorf("You cannot push a \"root\" repository. Please rename your repository to <user>/<repo> (ex: %s/%s)", username, repoInfo.LocalName)
	}

	v := url.Values{}
	v.Set("tag", tag)

	push := func(authConfig registry.AuthConfig) error {
		buf, err := json.Marshal(authConfig)
		if err != nil {
			return err
		}
		registryAuthHeader := []string{
			base64.URLEncoding.EncodeToString(buf),
		}

		return cli.stream("POST", "/images/"+remote+"/push?"+v.Encode(), nil, cli.out, map[string][]string{
			"X-Registry-Auth": registryAuthHeader,
		})
	}

	if err := push(authConfig); err != nil {
		if strings.Contains(err.Error(), "Status 401") {
			fmt.Fprintln(cli.out, "\nPlease login prior to push:")
			if err := cli.CmdLogin(repoInfo.Index.GetAuthConfigKey()); err != nil {
				return err
			}
			authConfig := cli.configFile.ResolveAuthConfig(repoInfo.Index)
			return push(authConfig)
		}
		return err
	}
	return nil
}

func (cli *DockerCli) CmdPull(args ...string) error {
	cmd := cli.Subcmd("pull", "NAME[:TAG|@DIGEST]", "Pull an image or a repository from the registry", true)
	allTags := cmd.Bool([]string{"a", "-all-tags"}, false, "Download all tagged images in the repository")
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)

	var (
		v         = url.Values{}
		remote    = cmd.Arg(0)
		newRemote = remote
	)
	taglessRemote, tag := parsers.ParseRepositoryTag(remote)
	if tag == "" && !*allTags {
		newRemote = utils.ImageReference(taglessRemote, graph.DEFAULTTAG)
	}
	if tag != "" && *allTags {
		return fmt.Errorf("tag can't be used with --all-tags/-a")
	}

	v.Set("fromImage", newRemote)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(taglessRemote)
	if err != nil {
		return err
	}

	cli.LoadConfigFile()

	// Resolve the Auth config relevant for this server
	authConfig := cli.configFile.ResolveAuthConfig(repoInfo.Index)

	pull := func(authConfig registry.AuthConfig) error {
		buf, err := json.Marshal(authConfig)
		if err != nil {
			return err
		}
		registryAuthHeader := []string{
			base64.URLEncoding.EncodeToString(buf),
		}

		return cli.stream("POST", "/images/create?"+v.Encode(), nil, cli.out, map[string][]string{
			"X-Registry-Auth": registryAuthHeader,
		})
	}

	if err := pull(authConfig); err != nil {
		if strings.Contains(err.Error(), "Status 401") {
			fmt.Fprintln(cli.out, "\nPlease login prior to pull:")
			if err := cli.CmdLogin(repoInfo.Index.GetAuthConfigKey()); err != nil {
				return err
			}
			authConfig := cli.configFile.ResolveAuthConfig(repoInfo.Index)
			return pull(authConfig)
		}
		return err
	}

	return nil
}

func (cli *DockerCli) CmdImages(args ...string) error {
	cmd := cli.Subcmd("images", "[REPOSITORY]", "List images", true)
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only show numeric IDs")
	all := cmd.Bool([]string{"a", "-all"}, false, "Show all images (default hides intermediate images)")
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")
	showDigests := cmd.Bool([]string{"-digests"}, false, "Show digests")
	// FIXME: --viz and --tree are deprecated. Remove them in a future version.
	flViz := cmd.Bool([]string{"#v", "#viz", "#-viz"}, false, "Output graph in graphviz format")
	flTree := cmd.Bool([]string{"#t", "#tree", "#-tree"}, false, "Output graph in tree format")

	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")
	cmd.Require(flag.Max, 1)

	utils.ParseFlags(cmd, args, true)

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process in the daemon/server.
	imageFilterArgs := filters.Args{}
	for _, f := range flFilter.GetAll() {
		var err error
		imageFilterArgs, err = filters.ParseFlag(f, imageFilterArgs)
		if err != nil {
			return err
		}
	}

	matchName := cmd.Arg(0)
	// FIXME: --viz and --tree are deprecated. Remove them in a future version.
	if *flViz || *flTree {
		v := url.Values{
			"all": []string{"1"},
		}
		if len(imageFilterArgs) > 0 {
			filterJson, err := filters.ToParam(imageFilterArgs)
			if err != nil {
				return err
			}
			v.Set("filters", filterJson)
		}

		body, _, err := readBody(cli.call("GET", "/images/json?"+v.Encode(), nil, false))
		if err != nil {
			return err
		}

		outs := engine.NewTable("Created", 0)
		if _, err := outs.ReadListFrom(body); err != nil {
			return err
		}

		var (
			printNode  func(cli *DockerCli, noTrunc bool, image *engine.Env, prefix string)
			startImage *engine.Env

			roots    = engine.NewTable("Created", outs.Len())
			byParent = make(map[string]*engine.Table)
		)

		for _, image := range outs.Data {
			if image.Get("ParentId") == "" {
				roots.Add(image)
			} else {
				if children, exists := byParent[image.Get("ParentId")]; exists {
					children.Add(image)
				} else {
					byParent[image.Get("ParentId")] = engine.NewTable("Created", 1)
					byParent[image.Get("ParentId")].Add(image)
				}
			}

			if matchName != "" {
				if matchName == image.Get("Id") || matchName == common.TruncateID(image.Get("Id")) {
					startImage = image
				}

				for _, repotag := range image.GetList("RepoTags") {
					if repotag == matchName {
						startImage = image
					}
				}
			}
		}

		if *flViz {
			fmt.Fprintf(cli.out, "digraph docker {\n")
			printNode = (*DockerCli).printVizNode
		} else {
			printNode = (*DockerCli).printTreeNode
		}

		if startImage != nil {
			root := engine.NewTable("Created", 1)
			root.Add(startImage)
			cli.WalkTree(*noTrunc, root, byParent, "", printNode)
		} else if matchName == "" {
			cli.WalkTree(*noTrunc, roots, byParent, "", printNode)
		}
		if *flViz {
			fmt.Fprintf(cli.out, " base [style=invisible]\n}\n")
		}
	} else {
		v := url.Values{}
		if len(imageFilterArgs) > 0 {
			filterJson, err := filters.ToParam(imageFilterArgs)
			if err != nil {
				return err
			}
			v.Set("filters", filterJson)
		}

		if cmd.NArg() == 1 {
			// FIXME rename this parameter, to not be confused with the filters flag
			v.Set("filter", matchName)
		}
		if *all {
			v.Set("all", "1")
		}

		body, _, err := readBody(cli.call("GET", "/images/json?"+v.Encode(), nil, false))

		if err != nil {
			return err
		}

		outs := engine.NewTable("Created", 0)
		if _, err := outs.ReadListFrom(body); err != nil {
			return err
		}

		w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
		if !*quiet {
			if *showDigests {
				fmt.Fprintln(w, "REPOSITORY\tTAG\tDIGEST\tIMAGE ID\tCREATED\tVIRTUAL SIZE")
			} else {
				fmt.Fprintln(w, "REPOSITORY\tTAG\tIMAGE ID\tCREATED\tVIRTUAL SIZE")
			}
		}

		for _, out := range outs.Data {
			outID := out.Get("Id")
			if !*noTrunc {
				outID = common.TruncateID(outID)
			}

			repoTags := out.GetList("RepoTags")
			repoDigests := out.GetList("RepoDigests")

			if len(repoTags) == 1 && repoTags[0] == "<none>:<none>" && len(repoDigests) == 1 && repoDigests[0] == "<none>@<none>" {
				// dangling image - clear out either repoTags or repoDigsts so we only show it once below
				repoDigests = []string{}
			}

			// combine the tags and digests lists
			tagsAndDigests := append(repoTags, repoDigests...)
			for _, repoAndRef := range tagsAndDigests {
				repo, ref := parsers.ParseRepositoryTag(repoAndRef)
				// default tag and digest to none - if there's a value, it'll be set below
				tag := "<none>"
				digest := "<none>"
				if utils.DigestReference(ref) {
					digest = ref
				} else {
					tag = ref
				}

				if !*quiet {
					if *showDigests {
						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s ago\t%s\n", repo, tag, digest, outID, units.HumanDuration(time.Now().UTC().Sub(time.Unix(out.GetInt64("Created"), 0))), units.HumanSize(float64(out.GetInt64("VirtualSize"))))
					} else {
						fmt.Fprintf(w, "%s\t%s\t%s\t%s ago\t%s\n", repo, tag, outID, units.HumanDuration(time.Now().UTC().Sub(time.Unix(out.GetInt64("Created"), 0))), units.HumanSize(float64(out.GetInt64("VirtualSize"))))
					}
				} else {
					fmt.Fprintln(w, outID)
				}
			}
		}

		if !*quiet {
			w.Flush()
		}
	}
	return nil
}

// FIXME: --viz and --tree are deprecated. Remove them in a future version.
func (cli *DockerCli) WalkTree(noTrunc bool, images *engine.Table, byParent map[string]*engine.Table, prefix string, printNode func(cli *DockerCli, noTrunc bool, image *engine.Env, prefix string)) {
	length := images.Len()
	if length > 1 {
		for index, image := range images.Data {
			if index+1 == length {
				printNode(cli, noTrunc, image, prefix+"└─")
				if subimages, exists := byParent[image.Get("Id")]; exists {
					cli.WalkTree(noTrunc, subimages, byParent, prefix+"  ", printNode)
				}
			} else {
				printNode(cli, noTrunc, image, prefix+"\u251C─")
				if subimages, exists := byParent[image.Get("Id")]; exists {
					cli.WalkTree(noTrunc, subimages, byParent, prefix+"\u2502 ", printNode)
				}
			}
		}
	} else {
		for _, image := range images.Data {
			printNode(cli, noTrunc, image, prefix+"└─")
			if subimages, exists := byParent[image.Get("Id")]; exists {
				cli.WalkTree(noTrunc, subimages, byParent, prefix+"  ", printNode)
			}
		}
	}
}

// FIXME: --viz and --tree are deprecated. Remove them in a future version.
func (cli *DockerCli) printVizNode(noTrunc bool, image *engine.Env, prefix string) {
	var (
		imageID  string
		parentID string
	)
	if noTrunc {
		imageID = image.Get("Id")
		parentID = image.Get("ParentId")
	} else {
		imageID = common.TruncateID(image.Get("Id"))
		parentID = common.TruncateID(image.Get("ParentId"))
	}
	if parentID == "" {
		fmt.Fprintf(cli.out, " base -> \"%s\" [style=invis]\n", imageID)
	} else {
		fmt.Fprintf(cli.out, " \"%s\" -> \"%s\"\n", parentID, imageID)
	}
	if image.GetList("RepoTags")[0] != "<none>:<none>" {
		fmt.Fprintf(cli.out, " \"%s\" [label=\"%s\\n%s\",shape=box,fillcolor=\"paleturquoise\",style=\"filled,rounded\"];\n",
			imageID, imageID, strings.Join(image.GetList("RepoTags"), "\\n"))
	}
}

// FIXME: --viz and --tree are deprecated. Remove them in a future version.
func (cli *DockerCli) printTreeNode(noTrunc bool, image *engine.Env, prefix string) {
	var imageID string
	if noTrunc {
		imageID = image.Get("Id")
	} else {
		imageID = common.TruncateID(image.Get("Id"))
	}

	fmt.Fprintf(cli.out, "%s%s Virtual Size: %s", prefix, imageID, units.HumanSize(float64(image.GetInt64("VirtualSize"))))
	if image.GetList("RepoTags")[0] != "<none>:<none>" {
		fmt.Fprintf(cli.out, " Tags: %s\n", strings.Join(image.GetList("RepoTags"), ", "))
	} else {
		fmt.Fprint(cli.out, "\n")
	}
}

func (cli *DockerCli) CmdPs(args ...string) error {
	var (
		err error

		psFilterArgs = filters.Args{}
		v            = url.Values{}

		cmd      = cli.Subcmd("ps", "", "List containers", true)
		quiet    = cmd.Bool([]string{"q", "-quiet"}, false, "Only display numeric IDs")
		size     = cmd.Bool([]string{"s", "-size"}, false, "Display total file sizes")
		all      = cmd.Bool([]string{"a", "-all"}, false, "Show all containers (default shows just running)")
		noTrunc  = cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")
		nLatest  = cmd.Bool([]string{"l", "-latest"}, false, "Show the latest created container, include non-running")
		since    = cmd.String([]string{"#sinceId", "#-since-id", "-since"}, "", "Show created since Id or Name, include non-running")
		before   = cmd.String([]string{"#beforeId", "#-before-id", "-before"}, "", "Show only container created before Id or Name")
		last     = cmd.Int([]string{"n"}, -1, "Show n last created containers, include non-running")
		flFilter = opts.NewListOpts(nil)
	)
	cmd.Require(flag.Exact, 0)

	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")

	utils.ParseFlags(cmd, args, true)
	if *last == -1 && *nLatest {
		*last = 1
	}

	if *all {
		v.Set("all", "1")
	}

	if *last != -1 {
		v.Set("limit", strconv.Itoa(*last))
	}

	if *since != "" {
		v.Set("since", *since)
	}

	if *before != "" {
		v.Set("before", *before)
	}

	if *size {
		v.Set("size", "1")
	}

	// Consolidate all filter flags, and sanity check them.
	// They'll get processed in the daemon/server.
	for _, f := range flFilter.GetAll() {
		if psFilterArgs, err = filters.ParseFlag(f, psFilterArgs); err != nil {
			return err
		}
	}

	if len(psFilterArgs) > 0 {
		filterJson, err := filters.ToParam(psFilterArgs)
		if err != nil {
			return err
		}

		v.Set("filters", filterJson)
	}

	body, _, err := readBody(cli.call("GET", "/containers/json?"+v.Encode(), nil, false))
	if err != nil {
		return err
	}

	outs := engine.NewTable("Created", 0)
	if _, err := outs.ReadListFrom(body); err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprint(w, "CONTAINER ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS\tPORTS\tNAMES")

		if *size {
			fmt.Fprintln(w, "\tSIZE")
		} else {
			fmt.Fprint(w, "\n")
		}
	}

	stripNamePrefix := func(ss []string) []string {
		for i, s := range ss {
			ss[i] = s[1:]
		}

		return ss
	}

	for _, out := range outs.Data {
		outID := out.Get("Id")

		if !*noTrunc {
			outID = common.TruncateID(outID)
		}

		if *quiet {
			fmt.Fprintln(w, outID)

			continue
		}

		var (
			outNames   = stripNamePrefix(out.GetList("Names"))
			outCommand = strconv.Quote(out.Get("Command"))
			ports      = engine.NewTable("", 0)
		)

		if !*noTrunc {
			outCommand = utils.Trunc(outCommand, 20)

			// only display the default name for the container with notrunc is passed
			for _, name := range outNames {
				if len(strings.Split(name, "/")) == 1 {
					outNames = []string{name}

					break
				}
			}
		}

		ports.ReadListFrom([]byte(out.Get("Ports")))

		image := out.Get("Image")
		if image == "" {
			image = "<no image>"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s ago\t%s\t%s\t%s\t", outID, image, outCommand,
			units.HumanDuration(time.Now().UTC().Sub(time.Unix(out.GetInt64("Created"), 0))),
			out.Get("Status"), api.DisplayablePorts(ports), strings.Join(outNames, ","))

		if *size {
			if out.GetInt("SizeRootFs") > 0 {
				fmt.Fprintf(w, "%s (virtual %s)\n", units.HumanSize(float64(out.GetInt64("SizeRw"))), units.HumanSize(float64(out.GetInt64("SizeRootFs"))))
			} else {
				fmt.Fprintf(w, "%s\n", units.HumanSize(float64(out.GetInt64("SizeRw"))))
			}

			continue
		}

		fmt.Fprint(w, "\n")
	}

	if !*quiet {
		w.Flush()
	}

	return nil
}

func (cli *DockerCli) CmdCommit(args ...string) error {
	cmd := cli.Subcmd("commit", "CONTAINER [REPOSITORY[:TAG]]", "Create a new image from a container's changes", true)
	flPause := cmd.Bool([]string{"p", "-pause"}, true, "Pause container during commit")
	flComment := cmd.String([]string{"m", "-message"}, "", "Commit message")
	flAuthor := cmd.String([]string{"a", "#author", "-author"}, "", "Author (e.g., \"John Hannibal Smith <hannibal@a-team.com>\")")
	flChanges := opts.NewListOpts(nil)
	cmd.Var(&flChanges, []string{"c", "-change"}, "Apply Dockerfile instruction to the created image")
	// FIXME: --run is deprecated, it will be replaced with inline Dockerfile commands.
	flConfig := cmd.String([]string{"#run", "#-run"}, "", "This option is deprecated and will be removed in a future version in favor of inline Dockerfile-compatible commands")
	cmd.Require(flag.Max, 2)
	cmd.Require(flag.Min, 1)
	utils.ParseFlags(cmd, args, true)

	var (
		name            = cmd.Arg(0)
		repository, tag = parsers.ParseRepositoryTag(cmd.Arg(1))
	)

	//Check if the given image name can be resolved
	if repository != "" {
		if err := registry.ValidateRepositoryName(repository); err != nil {
			return err
		}
	}

	v := url.Values{}
	v.Set("container", name)
	v.Set("repo", repository)
	v.Set("tag", tag)
	v.Set("comment", *flComment)
	v.Set("author", *flAuthor)
	for _, change := range flChanges.GetAll() {
		v.Add("changes", change)
	}

	if *flPause != true {
		v.Set("pause", "0")
	}

	var (
		config *runconfig.Config
		env    engine.Env
	)
	if *flConfig != "" {
		config = &runconfig.Config{}
		if err := json.Unmarshal([]byte(*flConfig), config); err != nil {
			return err
		}
	}
	stream, _, err := cli.call("POST", "/commit?"+v.Encode(), config, false)
	if err != nil {
		return err
	}
	if err := env.Decode(stream); err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "%s\n", env.Get("Id"))
	return nil
}

func (cli *DockerCli) CmdEvents(args ...string) error {
	cmd := cli.Subcmd("events", "", "Get real time events from the server", true)
	since := cmd.String([]string{"#since", "-since"}, "", "Show all events created since timestamp")
	until := cmd.String([]string{"-until"}, "", "Stream events until this timestamp")
	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")
	cmd.Require(flag.Exact, 0)

	utils.ParseFlags(cmd, args, true)

	var (
		v               = url.Values{}
		loc             = time.FixedZone(time.Now().Zone())
		eventFilterArgs = filters.Args{}
	)

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process in the daemon/server.
	for _, f := range flFilter.GetAll() {
		var err error
		eventFilterArgs, err = filters.ParseFlag(f, eventFilterArgs)
		if err != nil {
			return err
		}
	}
	var setTime = func(key, value string) {
		format := timeutils.RFC3339NanoFixed
		if len(value) < len(format) {
			format = format[:len(value)]
		}
		if t, err := time.ParseInLocation(format, value, loc); err == nil {
			v.Set(key, strconv.FormatInt(t.Unix(), 10))
		} else {
			v.Set(key, value)
		}
	}
	if *since != "" {
		setTime("since", *since)
	}
	if *until != "" {
		setTime("until", *until)
	}
	if len(eventFilterArgs) > 0 {
		filterJson, err := filters.ToParam(eventFilterArgs)
		if err != nil {
			return err
		}
		v.Set("filters", filterJson)
	}
	if err := cli.stream("GET", "/events?"+v.Encode(), nil, cli.out, nil); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdExport(args ...string) error {
	cmd := cli.Subcmd("export", "CONTAINER", "Export a filesystem as a tar archive (streamed to STDOUT by default)", true)
	outfile := cmd.String([]string{"o", "-output"}, "", "Write to a file, instead of STDOUT")
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)

	var (
		output io.Writer = cli.out
		err    error
	)
	if *outfile != "" {
		output, err = os.Create(*outfile)
		if err != nil {
			return err
		}
	} else if cli.isTerminalOut {
		return errors.New("Cowardly refusing to save to a terminal. Use the -o flag or redirect.")
	}

	if len(cmd.Args()) == 1 {
		image := cmd.Arg(0)
		if err := cli.stream("GET", "/containers/"+image+"/export", nil, output, nil); err != nil {
			return err
		}
	} else {
		v := url.Values{}
		for _, arg := range cmd.Args() {
			v.Add("names", arg)
		}
		if err := cli.stream("GET", "/containers/get?"+v.Encode(), nil, output, nil); err != nil {
			return err
		}
	}

	return nil
}

func (cli *DockerCli) CmdDiff(args ...string) error {
	cmd := cli.Subcmd("diff", "CONTAINER", "Inspect changes on a container's filesystem", true)
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)

	body, _, err := readBody(cli.call("GET", "/containers/"+cmd.Arg(0)+"/changes", nil, false))

	if err != nil {
		return err
	}

	outs := engine.NewTable("", 0)
	if _, err := outs.ReadListFrom(body); err != nil {
		return err
	}
	for _, change := range outs.Data {
		var kind string
		switch change.GetInt("Kind") {
		case archive.ChangeModify:
			kind = "C"
		case archive.ChangeAdd:
			kind = "A"
		case archive.ChangeDelete:
			kind = "D"
		}
		fmt.Fprintf(cli.out, "%s %s\n", kind, change.Get("Path"))
	}
	return nil
}

func (cli *DockerCli) CmdLogs(args ...string) error {
	var (
		cmd    = cli.Subcmd("logs", "CONTAINER", "Fetch the logs of a container", true)
		follow = cmd.Bool([]string{"f", "-follow"}, false, "Follow log output")
		times  = cmd.Bool([]string{"t", "-timestamps"}, false, "Show timestamps")
		tail   = cmd.String([]string{"-tail"}, "all", "Number of lines to show from the end of the logs")
	)
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)

	name := cmd.Arg(0)

	stream, _, err := cli.call("GET", "/containers/"+name+"/json", nil, false)
	if err != nil {
		return err
	}

	env := engine.Env{}
	if err := env.Decode(stream); err != nil {
		return err
	}

	if env.GetSubEnv("HostConfig").GetSubEnv("LogConfig").Get("Type") != "json-file" {
		return fmt.Errorf("\"logs\" command is supported only for \"json-file\" logging driver")
	}

	v := url.Values{}
	v.Set("stdout", "1")
	v.Set("stderr", "1")

	if *times {
		v.Set("timestamps", "1")
	}

	if *follow {
		v.Set("follow", "1")
	}
	v.Set("tail", *tail)

	return cli.streamHelper("GET", "/containers/"+name+"/logs?"+v.Encode(), env.GetSubEnv("Config").GetBool("Tty"), nil, cli.out, cli.err, nil)
}

func (cli *DockerCli) CmdAttach(args ...string) error {
	var (
		cmd     = cli.Subcmd("attach", "CONTAINER", "Attach to a running container", true)
		noStdin = cmd.Bool([]string{"#nostdin", "-no-stdin"}, false, "Do not attach STDIN")
		proxy   = cmd.Bool([]string{"#sig-proxy", "-sig-proxy"}, true, "Proxy all received signals to the process")
	)
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)
	name := cmd.Arg(0)

	stream, _, err := cli.call("GET", "/containers/"+name+"/json", nil, false)
	if err != nil {
		return err
	}

	env := engine.Env{}
	if err := env.Decode(stream); err != nil {
		return err
	}

	if !env.GetSubEnv("State").GetBool("Running") {
		return fmt.Errorf("You cannot attach to a stopped container, start it first")
	}

	var (
		config = env.GetSubEnv("Config")
		tty    = config.GetBool("Tty")
	)

	if err := cli.CheckTtyInput(!*noStdin, tty); err != nil {
		return err
	}

	if tty && cli.isTerminalOut {
		if err := cli.monitorTtySize(cmd.Arg(0), false); err != nil {
			log.Debugf("Error monitoring TTY size: %s", err)
		}
	}

	var in io.ReadCloser

	v := url.Values{}
	v.Set("stream", "1")
	if !*noStdin && config.GetBool("OpenStdin") {
		v.Set("stdin", "1")
		in = cli.in
	}

	v.Set("stdout", "1")
	v.Set("stderr", "1")

	if *proxy && !tty {
		sigc := cli.forwardAllSignals(cmd.Arg(0))
		defer signal.StopCatch(sigc)
	}

	if err := cli.hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?"+v.Encode(), tty, in, cli.out, cli.err, nil, nil); err != nil {
		return err
	}

	_, status, err := getExitCode(cli, cmd.Arg(0))
	if err != nil {
		return err
	}
	if status != 0 {
		return &utils.StatusError{StatusCode: status}
	}

	return nil
}

func (cli *DockerCli) CmdSearch(args ...string) error {
	cmd := cli.Subcmd("search", "TERM", "Search the Docker Hub for images", true)
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")
	trusted := cmd.Bool([]string{"#t", "#trusted", "#-trusted"}, false, "Only show trusted builds")
	automated := cmd.Bool([]string{"-automated"}, false, "Only show automated builds")
	stars := cmd.Int([]string{"s", "#stars", "-stars"}, 0, "Only displays with at least x stars")
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)

	v := url.Values{}
	v.Set("term", cmd.Arg(0))

	body, _, err := readBody(cli.call("GET", "/images/search?"+v.Encode(), nil, true))

	if err != nil {
		return err
	}
	outs := engine.NewTable("star_count", 0)
	if _, err := outs.ReadListFrom(body); err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 10, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tSTARS\tOFFICIAL\tAUTOMATED\n")
	for _, out := range outs.Data {
		if ((*automated || *trusted) && (!out.GetBool("is_trusted") && !out.GetBool("is_automated"))) || (*stars > out.GetInt("star_count")) {
			continue
		}
		desc := strings.Replace(out.Get("description"), "\n", " ", -1)
		desc = strings.Replace(desc, "\r", " ", -1)
		if !*noTrunc && len(desc) > 45 {
			desc = utils.Trunc(desc, 42) + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t", out.Get("name"), desc, out.GetInt("star_count"))
		if out.GetBool("is_official") {
			fmt.Fprint(w, "[OK]")

		}
		fmt.Fprint(w, "\t")
		if out.GetBool("is_automated") || out.GetBool("is_trusted") {
			fmt.Fprint(w, "[OK]")
		}
		fmt.Fprint(w, "\n")
	}
	w.Flush()
	return nil
}

// Ports type - Used to parse multiple -p flags
type ports []int

func (cli *DockerCli) CmdTag(args ...string) error {
	cmd := cli.Subcmd("tag", "IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]", "Tag an image into a repository", true)
	force := cmd.Bool([]string{"f", "#force", "-force"}, false, "Force")
	cmd.Require(flag.Exact, 2)

	utils.ParseFlags(cmd, args, true)

	var (
		repository, tag = parsers.ParseRepositoryTag(cmd.Arg(1))
		v               = url.Values{}
	)

	//Check if the given image name can be resolved
	if err := registry.ValidateRepositoryName(repository); err != nil {
		return err
	}
	v.Set("repo", repository)
	v.Set("tag", tag)

	if *force {
		v.Set("force", "1")
	}

	if _, _, err := readBody(cli.call("POST", "/images/"+cmd.Arg(0)+"/tag?"+v.Encode(), nil, false)); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) pullImage(image string) error {
	return cli.pullImageCustomOut(image, cli.out)
}

func (cli *DockerCli) pullImageCustomOut(image string, out io.Writer) error {
	v := url.Values{}
	repos, tag := parsers.ParseRepositoryTag(image)
	// pull only the image tagged 'latest' if no tag was specified
	if tag == "" {
		tag = graph.DEFAULTTAG
	}
	v.Set("fromImage", repos)
	v.Set("tag", tag)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(repos)
	if err != nil {
		return err
	}

	// Load the auth config file, to be able to pull the image
	cli.LoadConfigFile()

	// Resolve the Auth config relevant for this server
	authConfig := cli.configFile.ResolveAuthConfig(repoInfo.Index)
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}

	registryAuthHeader := []string{
		base64.URLEncoding.EncodeToString(buf),
	}
	if err = cli.stream("POST", "/images/create?"+v.Encode(), nil, out, map[string][]string{"X-Registry-Auth": registryAuthHeader}); err != nil {
		return err
	}
	return nil
}

type cidFile struct {
	path    string
	file    *os.File
	written bool
}

func newCIDFile(path string) (*cidFile, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("Container ID file found, make sure the other container isn't running or delete %s", path)
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to create the container ID file: %s", err)
	}

	return &cidFile{path: path, file: f}, nil
}

func (cid *cidFile) Close() error {
	cid.file.Close()

	if !cid.written {
		if err := os.Remove(cid.path); err != nil {
			return fmt.Errorf("failed to remove the CID file '%s': %s \n", cid.path, err)
		}
	}

	return nil
}

func (cid *cidFile) Write(id string) error {
	if _, err := cid.file.Write([]byte(id)); err != nil {
		return fmt.Errorf("Failed to write the container ID to the file: %s", err)
	}
	cid.written = true
	return nil
}

func (cli *DockerCli) createContainer(config *runconfig.Config, hostConfig *runconfig.HostConfig, cidfile, name string) (*types.ContainerCreateResponse, error) {
	containerValues := url.Values{}
	if name != "" {
		containerValues.Set("name", name)
	}

	mergedConfig := runconfig.MergeConfigs(config, hostConfig)

	var containerIDFile *cidFile
	if cidfile != "" {
		var err error
		if containerIDFile, err = newCIDFile(cidfile); err != nil {
			return nil, err
		}
		defer containerIDFile.Close()
	}

	//create the container
	stream, statusCode, err := cli.call("POST", "/containers/create?"+containerValues.Encode(), mergedConfig, false)
	//if image not found try to pull it
	if statusCode == 404 {
		repo, tag := parsers.ParseRepositoryTag(config.Image)
		if tag == "" {
			tag = graph.DEFAULTTAG
		}
		fmt.Fprintf(cli.err, "Unable to find image '%s' locally\n", utils.ImageReference(repo, tag))

		// we don't want to write to stdout anything apart from container.ID
		if err = cli.pullImageCustomOut(config.Image, cli.err); err != nil {
			return nil, err
		}
		// Retry
		if stream, _, err = cli.call("POST", "/containers/create?"+containerValues.Encode(), mergedConfig, false); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	var response types.ContainerCreateResponse
	if err := json.NewDecoder(stream).Decode(&response); err != nil {
		return nil, err
	}
	for _, warning := range response.Warnings {
		fmt.Fprintf(cli.err, "WARNING: %s\n", warning)
	}
	if containerIDFile != nil {
		if err = containerIDFile.Write(response.ID); err != nil {
			return nil, err
		}
	}
	return &response, nil
}

func (cli *DockerCli) CmdCreate(args ...string) error {
	cmd := cli.Subcmd("create", "IMAGE [COMMAND] [ARG...]", "Create a new container", true)

	// These are flags not stored in Config/HostConfig
	var (
		flName = cmd.String([]string{"-name"}, "", "Assign a name to the container")
	)

	config, hostConfig, cmd, err := runconfig.Parse(cmd, args)
	if err != nil {
		utils.ReportError(cmd, err.Error(), true)
	}
	if config.Image == "" {
		cmd.Usage()
		return nil
	}
	response, err := cli.createContainer(config, hostConfig, hostConfig.ContainerIDFile, *flName)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "%s\n", response.ID)
	return nil
}

func (cli *DockerCli) CmdRun(args ...string) error {
	// FIXME: just use runconfig.Parse already
	cmd := cli.Subcmd("run", "IMAGE [COMMAND] [ARG...]", "Run a command in a new container", true)

	// These are flags not stored in Config/HostConfig
	var (
		flAutoRemove = cmd.Bool([]string{"#rm", "-rm"}, false, "Automatically remove the container when it exits")
		flDetach     = cmd.Bool([]string{"d", "-detach"}, false, "Run container in background and print container ID")
		flSigProxy   = cmd.Bool([]string{"#sig-proxy", "-sig-proxy"}, true, "Proxy received signals to the process")
		flName       = cmd.String([]string{"#name", "-name"}, "", "Assign a name to the container")
		flAttach     *opts.ListOpts

		ErrConflictAttachDetach               = fmt.Errorf("Conflicting options: -a and -d")
		ErrConflictRestartPolicyAndAutoRemove = fmt.Errorf("Conflicting options: --restart and --rm")
		ErrConflictDetachAutoRemove           = fmt.Errorf("Conflicting options: --rm and -d")
	)

	config, hostConfig, cmd, err := runconfig.Parse(cmd, args)
	// just in case the Parse does not exit
	if err != nil {
		utils.ReportError(cmd, err.Error(), true)
	}

	if len(hostConfig.Dns) > 0 {
		// check the DNS settings passed via --dns against
		// localhost regexp to warn if they are trying to
		// set a DNS to a localhost address
		for _, dnsIP := range hostConfig.Dns {
			if resolvconf.IsLocalhost(dnsIP) {
				fmt.Fprintf(cli.err, "WARNING: Localhost DNS setting (--dns=%s) may fail in containers.\n", dnsIP)
				break
			}
		}
	}
	if config.Image == "" {
		cmd.Usage()
		return nil
	}

	if !*flDetach {
		if err := cli.CheckTtyInput(config.AttachStdin, config.Tty); err != nil {
			return err
		}
	} else {
		if fl := cmd.Lookup("-attach"); fl != nil {
			flAttach = fl.Value.(*opts.ListOpts)
			if flAttach.Len() != 0 {
				return ErrConflictAttachDetach
			}
		}
		if *flAutoRemove {
			return ErrConflictDetachAutoRemove
		}

		config.AttachStdin = false
		config.AttachStdout = false
		config.AttachStderr = false
		config.StdinOnce = false
	}

	// Disable flSigProxy when in TTY mode
	sigProxy := *flSigProxy
	if config.Tty {
		sigProxy = false
	}

	createResponse, err := cli.createContainer(config, hostConfig, hostConfig.ContainerIDFile, *flName)
	if err != nil {
		return err
	}
	if sigProxy {
		sigc := cli.forwardAllSignals(createResponse.ID)
		defer signal.StopCatch(sigc)
	}
	var (
		waitDisplayId chan struct{}
		errCh         chan error
	)
	if !config.AttachStdout && !config.AttachStderr {
		// Make this asynchronous to allow the client to write to stdin before having to read the ID
		waitDisplayId = make(chan struct{})
		go func() {
			defer close(waitDisplayId)
			fmt.Fprintf(cli.out, "%s\n", createResponse.ID)
		}()
	}
	if *flAutoRemove && (hostConfig.RestartPolicy.Name == "always" || hostConfig.RestartPolicy.Name == "on-failure") {
		return ErrConflictRestartPolicyAndAutoRemove
	}
	// We need to instantiate the chan because the select needs it. It can
	// be closed but can't be uninitialized.
	hijacked := make(chan io.Closer)
	// Block the return until the chan gets closed
	defer func() {
		log.Debugf("End of CmdRun(), Waiting for hijack to finish.")
		if _, ok := <-hijacked; ok {
			log.Errorf("Hijack did not finish (chan still open)")
		}
	}()
	if config.AttachStdin || config.AttachStdout || config.AttachStderr {
		var (
			out, stderr io.Writer
			in          io.ReadCloser
			v           = url.Values{}
		)
		v.Set("stream", "1")
		if config.AttachStdin {
			v.Set("stdin", "1")
			in = cli.in
		}
		if config.AttachStdout {
			v.Set("stdout", "1")
			out = cli.out
		}
		if config.AttachStderr {
			v.Set("stderr", "1")
			if config.Tty {
				stderr = cli.out
			} else {
				stderr = cli.err
			}
		}
		errCh = promise.Go(func() error {
			return cli.hijack("POST", "/containers/"+createResponse.ID+"/attach?"+v.Encode(), config.Tty, in, out, stderr, hijacked, nil)
		})
	} else {
		close(hijacked)
	}
	// Acknowledge the hijack before starting
	select {
	case closer := <-hijacked:
		// Make sure that the hijack gets closed when returning (results
		// in closing the hijack chan and freeing server's goroutines)
		if closer != nil {
			defer closer.Close()
		}
	case err := <-errCh:
		if err != nil {
			log.Debugf("Error hijack: %s", err)
			return err
		}
	}

	//start the container
	if _, _, err = readBody(cli.call("POST", "/containers/"+createResponse.ID+"/start", nil, false)); err != nil {
		return err
	}

	if (config.AttachStdin || config.AttachStdout || config.AttachStderr) && config.Tty && cli.isTerminalOut {
		if err := cli.monitorTtySize(createResponse.ID, false); err != nil {
			log.Errorf("Error monitoring TTY size: %s", err)
		}
	}

	if errCh != nil {
		if err := <-errCh; err != nil {
			log.Debugf("Error hijack: %s", err)
			return err
		}
	}

	// Detached mode: wait for the id to be displayed and return.
	if !config.AttachStdout && !config.AttachStderr {
		// Detached mode
		<-waitDisplayId
		return nil
	}

	var status int

	// Attached mode
	if *flAutoRemove {
		// Autoremove: wait for the container to finish, retrieve
		// the exit code and remove the container
		if _, _, err := readBody(cli.call("POST", "/containers/"+createResponse.ID+"/wait", nil, false)); err != nil {
			return err
		}
		if _, status, err = getExitCode(cli, createResponse.ID); err != nil {
			return err
		}
		if _, _, err := readBody(cli.call("DELETE", "/containers/"+createResponse.ID+"?v=1", nil, false)); err != nil {
			return err
		}
	} else {
		// No Autoremove: Simply retrieve the exit code
		if !config.Tty {
			// In non-TTY mode, we can't detach, so we must wait for container exit
			if status, err = waitForExit(cli, createResponse.ID); err != nil {
				return err
			}
		} else {
			// In TTY mode, there is a race: if the process dies too slowly, the state could
			// be updated after the getExitCode call and result in the wrong exit code being reported
			if _, status, err = getExitCode(cli, createResponse.ID); err != nil {
				return err
			}
		}
	}
	if status != 0 {
		return &utils.StatusError{StatusCode: status}
	}
	return nil
}

func (cli *DockerCli) CmdCp(args ...string) error {
	cmd := cli.Subcmd("cp", "CONTAINER:PATH HOSTDIR|-", "Copy files/folders from a PATH on the container to a HOSTDIR on the host\nrunning the command. Use '-' to write the data\nas a tar file to STDOUT.", true)
	cmd.Require(flag.Exact, 2)

	utils.ParseFlags(cmd, args, true)

	var copyData engine.Env
	info := strings.Split(cmd.Arg(0), ":")

	if len(info) != 2 {
		return fmt.Errorf("Error: Path not specified")
	}

	copyData.Set("Resource", info[1])
	copyData.Set("HostPath", cmd.Arg(1))

	stream, statusCode, err := cli.call("POST", "/containers/"+info[0]+"/copy", copyData, false)
	if stream != nil {
		defer stream.Close()
	}
	if statusCode == 404 {
		return fmt.Errorf("No such container: %v", info[0])
	}
	if err != nil {
		return err
	}

	if statusCode == 200 {
		dest := copyData.Get("HostPath")

		if dest == "-" {
			_, err = io.Copy(cli.out, stream)
		} else {
			err = archive.Untar(stream, dest, &archive.TarOptions{NoLchown: true})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (cli *DockerCli) CmdSave(args ...string) error {
	cmd := cli.Subcmd("save", "IMAGE [IMAGE...]", "Save an image(s) to a tar archive (streamed to STDOUT by default)", true)
	outfile := cmd.String([]string{"o", "-output"}, "", "Write to an file, instead of STDOUT")
	cmd.Require(flag.Min, 1)

	utils.ParseFlags(cmd, args, true)

	var (
		output io.Writer = cli.out
		err    error
	)
	if *outfile != "" {
		output, err = os.Create(*outfile)
		if err != nil {
			return err
		}
	} else if cli.isTerminalOut {
		return errors.New("Cowardly refusing to save to a terminal. Use the -o flag or redirect.")
	}

	if len(cmd.Args()) == 1 {
		image := cmd.Arg(0)
		if err := cli.stream("GET", "/images/"+image+"/get", nil, output, nil); err != nil {
			return err
		}
	} else {
		v := url.Values{}
		for _, arg := range cmd.Args() {
			v.Add("names", arg)
		}
		if err := cli.stream("GET", "/images/get?"+v.Encode(), nil, output, nil); err != nil {
			return err
		}
	}
	return nil
}

func (cli *DockerCli) CmdLoad(args ...string) error {
	cmd := cli.Subcmd("load", "", "Load an image from a tar archive on STDIN", true)
	infile := cmd.String([]string{"i", "-input"}, "", "Read from a tar archive file, instead of STDIN")
	cmd.Require(flag.Exact, 0)

	utils.ParseFlags(cmd, args, true)

	var (
		input io.Reader = cli.in
		err   error
	)
	if *infile != "" {
		input, err = os.Open(*infile)
		if err != nil {
			return err
		}
	}
	if err := cli.stream("POST", "/images/load", input, cli.out, nil); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdExec(args ...string) error {
	cmd := cli.Subcmd("exec", "CONTAINER COMMAND [ARG...]", "Run a command in a running container", true)

	execConfig, err := runconfig.ParseExec(cmd, args)
	// just in case the ParseExec does not exit
	if execConfig.Container == "" || err != nil {
		return &utils.StatusError{StatusCode: 1}
	}

	stream, _, err := cli.call("POST", "/containers/"+execConfig.Container+"/exec", execConfig, false)
	if err != nil {
		return err
	}

	var execResult engine.Env
	if err := execResult.Decode(stream); err != nil {
		return err
	}

	execID := execResult.Get("Id")

	if execID == "" {
		fmt.Fprintf(cli.out, "exec ID empty")
		return nil
	}

	if !execConfig.Detach {
		if err := cli.CheckTtyInput(execConfig.AttachStdin, execConfig.Tty); err != nil {
			return err
		}
	} else {
		if _, _, err := readBody(cli.call("POST", "/exec/"+execID+"/start", execConfig, false)); err != nil {
			return err
		}
		// For now don't print this - wait for when we support exec wait()
		// fmt.Fprintf(cli.out, "%s\n", execID)
		return nil
	}

	// Interactive exec requested.
	var (
		out, stderr io.Writer
		in          io.ReadCloser
		hijacked    = make(chan io.Closer)
		errCh       chan error
	)

	// Block the return until the chan gets closed
	defer func() {
		log.Debugf("End of CmdExec(), Waiting for hijack to finish.")
		if _, ok := <-hijacked; ok {
			log.Errorf("Hijack did not finish (chan still open)")
		}
	}()

	if execConfig.AttachStdin {
		in = cli.in
	}
	if execConfig.AttachStdout {
		out = cli.out
	}
	if execConfig.AttachStderr {
		if execConfig.Tty {
			stderr = cli.out
		} else {
			stderr = cli.err
		}
	}
	errCh = promise.Go(func() error {
		return cli.hijack("POST", "/exec/"+execID+"/start", execConfig.Tty, in, out, stderr, hijacked, execConfig)
	})

	// Acknowledge the hijack before starting
	select {
	case closer := <-hijacked:
		// Make sure that hijack gets closed when returning. (result
		// in closing hijack chan and freeing server's goroutines.
		if closer != nil {
			defer closer.Close()
		}
	case err := <-errCh:
		if err != nil {
			log.Debugf("Error hijack: %s", err)
			return err
		}
	}

	if execConfig.Tty && cli.isTerminalIn {
		if err := cli.monitorTtySize(execID, true); err != nil {
			log.Errorf("Error monitoring TTY size: %s", err)
		}
	}

	if err := <-errCh; err != nil {
		log.Debugf("Error hijack: %s", err)
		return err
	}

	var status int
	if _, status, err = getExecExitCode(cli, execID); err != nil {
		return err
	}

	if status != 0 {
		return &utils.StatusError{StatusCode: status}
	}

	return nil
}

type containerStats struct {
	Name             string
	CpuPercentage    float64
	Memory           float64
	MemoryLimit      float64
	MemoryPercentage float64
	NetworkRx        float64
	NetworkTx        float64
	mu               sync.RWMutex
	err              error
}

func (s *containerStats) Collect(cli *DockerCli) {
	stream, _, err := cli.call("GET", "/containers/"+s.Name+"/stats", nil, false)
	if err != nil {
		s.err = err
		return
	}
	defer stream.Close()
	var (
		previousCpu    uint64
		previousSystem uint64
		start          = true
		dec            = json.NewDecoder(stream)
		u              = make(chan error, 1)
	)
	go func() {
		for {
			var v *types.Stats
			if err := dec.Decode(&v); err != nil {
				u <- err
				return
			}
			var (
				memPercent = float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit) * 100.0
				cpuPercent = 0.0
			)
			if !start {
				cpuPercent = calculateCpuPercent(previousCpu, previousSystem, v)
			}
			start = false
			s.mu.Lock()
			s.CpuPercentage = cpuPercent
			s.Memory = float64(v.MemoryStats.Usage)
			s.MemoryLimit = float64(v.MemoryStats.Limit)
			s.MemoryPercentage = memPercent
			s.NetworkRx = float64(v.Network.RxBytes)
			s.NetworkTx = float64(v.Network.TxBytes)
			s.mu.Unlock()
			previousCpu = v.CpuStats.CpuUsage.TotalUsage
			previousSystem = v.CpuStats.SystemUsage
			u <- nil
		}
	}()
	for {
		select {
		case <-time.After(2 * time.Second):
			// zero out the values if we have not received an update within
			// the specified duration.
			s.mu.Lock()
			s.CpuPercentage = 0
			s.Memory = 0
			s.MemoryPercentage = 0
			s.mu.Unlock()
		case err := <-u:
			if err != nil {
				s.mu.Lock()
				s.err = err
				s.mu.Unlock()
				return
			}
		}
	}
}

func (s *containerStats) Display(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.err != nil {
		return s.err
	}
	fmt.Fprintf(w, "%s\t%.2f%%\t%s/%s\t%.2f%%\t%s/%s\n",
		s.Name,
		s.CpuPercentage,
		units.BytesSize(s.Memory), units.BytesSize(s.MemoryLimit),
		s.MemoryPercentage,
		units.BytesSize(s.NetworkRx), units.BytesSize(s.NetworkTx))
	return nil
}

func (cli *DockerCli) CmdStats(args ...string) error {
	cmd := cli.Subcmd("stats", "CONTAINER [CONTAINER...]", "Display a live stream of one or more containers' resource usage statistics", true)
	cmd.Require(flag.Min, 1)
	utils.ParseFlags(cmd, args, true)

	names := cmd.Args()
	sort.Strings(names)
	var (
		cStats []*containerStats
		w      = tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	)
	printHeader := func() {
		fmt.Fprint(cli.out, "\033[2J")
		fmt.Fprint(cli.out, "\033[H")
		fmt.Fprintln(w, "CONTAINER\tCPU %\tMEM USAGE/LIMIT\tMEM %\tNET I/O")
	}
	for _, n := range names {
		s := &containerStats{Name: n}
		cStats = append(cStats, s)
		go s.Collect(cli)
	}
	// do a quick pause so that any failed connections for containers that do not exist are able to be
	// evicted before we display the initial or default values.
	time.Sleep(500 * time.Millisecond)
	var errs []string
	for _, c := range cStats {
		c.mu.Lock()
		if c.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.Name, c.err))
		}
		c.mu.Unlock()
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, ", "))
	}
	for _ = range time.Tick(500 * time.Millisecond) {
		printHeader()
		toRemove := []int{}
		for i, s := range cStats {
			if err := s.Display(w); err != nil {
				toRemove = append(toRemove, i)
			}
		}
		for j := len(toRemove) - 1; j >= 0; j-- {
			i := toRemove[j]
			cStats = append(cStats[:i], cStats[i+1:]...)
		}
		if len(cStats) == 0 {
			return nil
		}
		w.Flush()
	}
	return nil
}

func calculateCpuPercent(previousCpu, previousSystem uint64, v *types.Stats) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CpuStats.CpuUsage.TotalUsage - previousCpu)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.CpuStats.SystemUsage - previousSystem)
	)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CpuStats.CpuUsage.PercpuUsage)) * 100.0
	}
	return cpuPercent
}
