package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"text/tabwriter"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
)

func (cli *DockerCli) listRemoteTags(names ...string) error {
	var (
		err          error
		rdr          io.ReadCloser
		repoTagLists []*types.RepositoryTagList
	)

	for _, name := range names {
		_, tag := parsers.ParseRepositoryTag(name)
		if tag != "" {
			logrus.Warnf("Skipping repository %q due to excessive tag %q.", name, tag)
			continue
		}
		// Resolve the Repository name from fqn to RepositoryInfo
		repoInfo, err := registry.ParseRepositoryInfo(name)
		if err != nil {
			logrus.Warnf("Failed to parse repository info %q: %v", name, err)
			continue
		}
		rdr, _, err = cli.clientRequestAttemptLogin("GET", "/images/"+name+"/tags", nil, nil, repoInfo.Index, "tag")
		if err != nil {
			logrus.Warnf("Failed to get remote tag list for %q: %v", name, err)
			continue
		}

		tagList := types.RepositoryTagList{}
		if err := json.NewDecoder(rdr).Decode(&tagList); err != nil {
			logrus.Warnf("Failed to decode remote tag list for %q: %v", name, err)
			continue
		}
		repoTagLists = append(repoTagLists, &tagList)
	}

	if len(repoTagLists) < 1 && err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, "REPOSITORY\tTAG\tIMAGE ID")

	for _, tagList := range repoTagLists {
		for _, tag := range tagList.TagList {
			fmt.Fprintf(w, "%s\t%s\t%s\n", tagList.Name, tag.Tag, tag.ImageID)
		}
	}

	w.Flush()
	return nil
}

// CmdTag tags an image into a repository.
//
// Usage: docker tag [OPTIONS] IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]
func (cli *DockerCli) CmdTag(args ...string) error {
	cmd := cli.Subcmd("tag", []string{
		"IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]",
		"-l [REGISTRYHOST/][USERNAME/]NAME...",
	}, "Tag an image or list remote tags", true)
	force := cmd.Bool([]string{"f", "#force", "-force"}, false, "Force")
	list := cmd.Bool([]string{"l", "#list", "-list"}, false, "List tags of remote repositories")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)
	if !*list && cmd.NArg() != 2 {
		cmd.ReportError("Tag command requires exactly 2 arguments.", true)
	}

	if *list {
		return cli.listRemoteTags(cmd.Args()...)
	} else {
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
		if _, _, err := readBody(cli.call("POST", "/images/"+cmd.Arg(0)+"/tag?"+v.Encode(), nil, nil)); err != nil {
			return err
		}
	}
	return nil
}
