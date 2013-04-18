package docker

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"text/tabwriter"
)

func ParseCommands(args []string) error {

	cmds := map[string]func(args []string) error{
		"images":  cmdImages,
		"kill":    cmdKill,
		"version": cmdVersion,
	}

	if len(args) > 0 {
		cmd, exists := cmds[args[0]]
		if !exists {
			//TODO display commend not found
			return cmdHelp(args)
		}
		return cmd(args[1:])
	}
	return cmdHelp(args)
}

func cmdHelp(args []string) error {
	help := "Usage: docker COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n"
	for _, cmd := range [][]string{
		//		{"attach", "Attach to a running container"},
		//		{"commit", "Create a new image from a container's changes"},
		//		{"diff", "Inspect changes on a container's filesystem"},
		//		{"export", "Stream the contents of a container as a tar archive"},
		//		{"history", "Show the history of an image"},
		{"images", "List images"},
		//		{"import", "Create a new filesystem image from the contents of a tarball"},
		//		{"info", "Display system-wide information"},
		//		{"inspect", "Return low-level information on a container"},
		{"kill", "Kill a running container"},
		//		{"login", "Register or Login to the docker registry server"},
		//		{"logs", "Fetch the logs of a container"},
		//		{"port", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT"},
		//		{"ps", "List containers"},
		//		{"pull", "Pull an image or a repository from the docker registry server"},
		//		{"push", "Push an image or a repository to the docker registry server"},
		//		{"restart", "Restart a running container"},
		//		{"rm", "Remove a container"},
		//		{"rmi", "Remove an image"},
		//		{"run", "Run a command in a new container"},
		//		{"start", "Start a stopped container"},
		//		{"stop", "Stop a running container"},
		//		{"tag", "Tag an image into a repository"},
		{"version", "Show the docker version information"},
		//		{"wait", "Block until a container stops, then print its exit code"},
	} {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd[0], cmd[1])
	}
	fmt.Println(help)
	return nil
}

func cmdImages(args []string) error {
	cmd := subcmd("images", "[OPTIONS] [NAME]", "List images")
	quiet := cmd.Bool("q", false, "only show numeric IDs")
	flAll := cmd.Bool("a", false, "show all images")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 1 {
		cmd.Usage()
		return nil
	}
	var nameFilter string
	if cmd.NArg() == 1 {
		nameFilter = cmd.Arg(0)
	}

	in := ImagesIn{nameFilter, *quiet, *flAll}

	body, err := apiPost("images", in)
	if err != nil {
		return err
	}

	var outs []ImagesOut
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprintln(w, "REPOSITORY\tTAG\tID\tCREATED")
	}

	for _, out := range outs {
		if !*quiet {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", out.Repository, out.Tag, out.Id, out.Created)
		} else {
			fmt.Fprintln(w, out.Id)
		}
	}

	if !*quiet {
		w.Flush()
	}
	return nil

}

func cmdKill(args []string) error {
	cmd := subcmd("kill", "CONTAINER [CONTAINER...]", "Kill a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	body, err := apiPost("kill", args)
	if err != nil {
		return err
	}

	var out SimpleMessage
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	fmt.Print(out.Message)
	return nil
}

func cmdVersion(_ []string) error {
	body, err := apiGet("version")
	if err != nil {
		return err
	}

	var out VersionOut
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	fmt.Println("Version:", out.Version)
	fmt.Println("Git Commit:", out.GitCommit)
	if out.MemoryLimitDisabled {
		fmt.Println("Memory limit disabled")
	}

	return nil
}

func apiGet(path string) ([]byte, error) {
	resp, err := http.Get("http://0.0.0.0:4243/" + path)
	if err != nil {
		return nil, err
	}
	//TODO check status code
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil

}

func apiPost(path string, data interface{}) ([]byte, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	dataBuf := bytes.NewBuffer(buf)
	resp, err := http.Post("http://0.0.0.0:4243/"+path, "application/json", dataBuf)
	if err != nil {
		return nil, err
	}
	//TODO check status code
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil

}

func subcmd(name, signature, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Printf("\nUsage: docker %s %s\n\n%s\n\n", name, signature, description)
		flags.PrintDefaults()
	}
	return flags
}
