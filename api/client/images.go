package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/units"
	"github.com/docker/docker/utils"
)

// FIXME: --viz and --tree are deprecated. Remove them in a future version.
func (cli *DockerCli) walkTree(noTrunc bool, images []*types.Image, byParent map[string][]*types.Image, prefix string, printNode func(cli *DockerCli, noTrunc bool, image *types.Image, prefix string)) {
	length := len(images)
	if length > 1 {
		for index, image := range images {
			if index+1 == length {
				printNode(cli, noTrunc, image, prefix+"└─")
				if subimages, exists := byParent[image.ID]; exists {
					cli.walkTree(noTrunc, subimages, byParent, prefix+"  ", printNode)
				}
			} else {
				printNode(cli, noTrunc, image, prefix+"\u251C─")
				if subimages, exists := byParent[image.ID]; exists {
					cli.walkTree(noTrunc, subimages, byParent, prefix+"\u2502 ", printNode)
				}
			}
		}
	} else {
		for _, image := range images {
			printNode(cli, noTrunc, image, prefix+"└─")
			if subimages, exists := byParent[image.ID]; exists {
				cli.walkTree(noTrunc, subimages, byParent, prefix+"  ", printNode)
			}
		}
	}
}

// FIXME: --viz and --tree are deprecated. Remove them in a future version.
func (cli *DockerCli) printVizNode(noTrunc bool, image *types.Image, prefix string) {
	var (
		imageID  string
		parentID string
	)
	if noTrunc {
		imageID = image.ID
		parentID = image.ParentId
	} else {
		imageID = stringid.TruncateID(image.ID)
		parentID = stringid.TruncateID(image.ParentId)
	}
	if parentID == "" {
		fmt.Fprintf(cli.out, " base -> \"%s\" [style=invis]\n", imageID)
	} else {
		fmt.Fprintf(cli.out, " \"%s\" -> \"%s\"\n", parentID, imageID)
	}
	if image.RepoTags[0] != "<none>:<none>" {
		fmt.Fprintf(cli.out, " \"%s\" [label=\"%s\\n%s\",shape=box,fillcolor=\"paleturquoise\",style=\"filled,rounded\"];\n",
			imageID, imageID, strings.Join(image.RepoTags, "\\n"))
	}
}

// FIXME: --viz and --tree are deprecated. Remove them in a future version.
func (cli *DockerCli) printTreeNode(noTrunc bool, image *types.Image, prefix string) {
	var imageID string
	if noTrunc {
		imageID = image.ID
	} else {
		imageID = stringid.TruncateID(image.ID)
	}

	fmt.Fprintf(cli.out, "%s%s Virtual Size: %s", prefix, imageID, units.HumanSize(float64(image.VirtualSize)))
	if image.RepoTags[0] != "<none>:<none>" {
		fmt.Fprintf(cli.out, " Tags: %s\n", strings.Join(image.RepoTags, ", "))
	} else {
		fmt.Fprint(cli.out, "\n")
	}
}

// CmdImages lists the images in a specified repository, or all top-level images if no repository is specified.
//
// Usage: docker images [OPTIONS] [REPOSITORY]
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
	cmd.ParseFlags(args, true)

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
			filterJSON, err := filters.ToParam(imageFilterArgs)
			if err != nil {
				return err
			}
			v.Set("filters", filterJSON)
		}

		rdr, _, err := cli.call("GET", "/images/json?"+v.Encode(), nil, nil)
		if err != nil {
			return err
		}

		images := []types.Image{}
		err = json.NewDecoder(rdr).Decode(&images)
		if err != nil {
			return err
		}

		var (
			printNode  func(cli *DockerCli, noTrunc bool, image *types.Image, prefix string)
			startImage *types.Image

			roots    = []*types.Image{}
			byParent = make(map[string][]*types.Image)
		)

		for _, image := range images {
			if image.ParentId == "" {
				roots = append(roots, &image)
			} else {
				if children, exists := byParent[image.ParentId]; exists {
					children = append(children, &image)
				} else {
					byParent[image.ParentId] = []*types.Image{&image}
				}
			}

			if matchName != "" {
				if matchName == image.ID || matchName == stringid.TruncateID(image.ID) {
					startImage = &image
				}

				for _, repotag := range image.RepoTags {
					if repotag == matchName {
						startImage = &image
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
			root := []*types.Image{startImage}
			cli.walkTree(*noTrunc, root, byParent, "", printNode)
		} else if matchName == "" {
			cli.walkTree(*noTrunc, roots, byParent, "", printNode)
		}
		if *flViz {
			fmt.Fprintf(cli.out, " base [style=invisible]\n}\n")
		}
	} else {
		v := url.Values{}
		if len(imageFilterArgs) > 0 {
			filterJSON, err := filters.ToParam(imageFilterArgs)
			if err != nil {
				return err
			}
			v.Set("filters", filterJSON)
		}

		if cmd.NArg() == 1 {
			// FIXME rename this parameter, to not be confused with the filters flag
			v.Set("filter", matchName)
		}
		if *all {
			v.Set("all", "1")
		}

		rdr, _, err := cli.call("GET", "/images/json?"+v.Encode(), nil, nil)
		if err != nil {
			return err
		}

		images := []types.Image{}
		err = json.NewDecoder(rdr).Decode(&images)
		if err != nil {
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

		for _, image := range images {
			ID := image.ID
			if !*noTrunc {
				ID = stringid.TruncateID(ID)
			}

			repoTags := image.RepoTags
			repoDigests := image.RepoDigests

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
						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s ago\t%s\n", repo, tag, digest, ID, units.HumanDuration(time.Now().UTC().Sub(time.Unix(int64(image.Created), 0))), units.HumanSize(float64(image.VirtualSize)))
					} else {
						fmt.Fprintf(w, "%s\t%s\t%s\t%s ago\t%s\n", repo, tag, ID, units.HumanDuration(time.Now().UTC().Sub(time.Unix(int64(image.Created), 0))), units.HumanSize(float64(image.VirtualSize)))
					}
				} else {
					fmt.Fprintln(w, ID)
				}
			}
		}

		if !*quiet {
			w.Flush()
		}
	}
	return nil
}
