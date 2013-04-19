package docker

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"text/tabwriter"
)

func ParseCommands(args []string) error {

	cmds := map[string]func(args []string) error{
		"images":  cmdImages,
		"info":    cmdInfo,
		"history": cmdHistory,
		"kill":    cmdKill,
		"logs":    cmdLogs,
		"ps":      cmdPs,
		"restart": cmdRestart,
		"rm":      cmdRm,
		"rmi":     cmdRmi,
		"run":     cmdRun,
		"start":   cmdStart,
		"stop":    cmdStop,
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
		{"history", "Show the history of an image"},
		{"images", "List images"},
		//		{"import", "Create a new filesystem image from the contents of a tarball"},
		{"info", "Display system-wide information"},
		//		{"inspect", "Return low-level information on a container"},
		{"kill", "Kill a running container"},
		//		{"login", "Register or Login to the docker registry server"},
		{"logs", "Fetch the logs of a container"},
		//		{"port", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT"},
		{"ps", "List containers"},
		//		{"pull", "Pull an image or a repository from the docker registry server"},
		//		{"push", "Push an image or a repository to the docker registry server"},
		{"restart", "Restart a running container"},
		{"rm", "Remove a container"},
		{"rmi", "Remove an image"},
		{"run", "Run a command in a new container"},
		{"start", "Start a stopped container"},
		{"stop", "Stop a running container"},
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
	cmd := Subcmd("images", "[OPTIONS] [NAME]", "List images")
	var in ImagesIn
	cmd.BoolVar(&in.Quiet, "q", false, "only show numeric IDs")
	cmd.BoolVar(&in.All, "a", false, "show all images")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 1 {
		cmd.Usage()
		return nil
	}
	if cmd.NArg() == 1 {
		in.NameFilter = cmd.Arg(0)
	}

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
	if !in.Quiet {
		fmt.Fprintln(w, "REPOSITORY\tTAG\tID\tCREATED")
	}

	for _, out := range outs {
		if !in.Quiet {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", out.Repository, out.Tag, out.Id, out.Created)
		} else {
			fmt.Fprintln(w, out.Id)
		}
	}

	if !in.Quiet {
		w.Flush()
	}
	return nil

}

func cmdInfo(args []string) error {
	cmd := Subcmd("info", "", "Display system-wide information")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	body, err := apiGet("info")
	if err != nil {
		return err
	}

	var out InfoOut
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	fmt.Printf("containers: %d\nversion: %s\nimages: %d\n", out.Containers, out.Version, out.Images)
	if out.Debug {
		fmt.Println("debug mode enabled")
		fmt.Printf("fds: %d\ngoroutines: %d\n", out.NFd, out.NGoroutines)
	}
	return nil

}

func cmdHistory(args []string) error {
	cmd := Subcmd("history", "IMAGE", "Show the history of an image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, err := apiPost("history", HistoryIn{cmd.Arg(0)})
	if err != nil {
		return err
	}

	var outs []HistoryOut
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tCREATED\tCREATED BY")

	for _, out := range outs {
		fmt.Fprintf(w, "%s\t%s\t%s\n", out.Id, out.Created, out.CreatedBy)
	}
	w.Flush()
	return nil
}

func cmdKill(args []string) error {
	cmd := Subcmd("kill", "CONTAINER [CONTAINER...]", "Kill a running container")
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

func cmdLogs(args []string) error {
	cmd := Subcmd("logs", "CONTAINER", "Fetch the logs of a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	body, err := apiPost("logs", LogsIn{cmd.Arg(0)})
	if err != nil {
		return err
	}

	var out LogsOut
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, out.Stdout)
	fmt.Fprintln(os.Stderr, out.Stderr)

	return nil
}

func cmdPs(args []string) error {
	cmd := Subcmd("ps", "[OPTIONS]", "List containers")
	var in PsIn
	cmd.BoolVar(&in.Quiet, "q", false, "Only display numeric IDs")
	cmd.BoolVar(&in.All, "a", false, "Show all containers. Only running containers are shown by default.")
	cmd.BoolVar(&in.Full, "notrunc", false, "Don't truncate output")
	nLatest := cmd.Bool("l", false, "Show only the latest created container, include non-running ones.")
	cmd.IntVar(&in.Last, "n", -1, "Show n last created containers, include non-running ones.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if in.Last == -1 && *nLatest {
		in.Last = 1
	}

	body, err := apiPost("ps", in)
	if err != nil {
		return err
	}

	var outs []PsOut
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	if !in.Quiet {
		fmt.Fprintln(w, "ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS")
	}

	for _, out := range outs {
		if !in.Quiet {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", out.Id, out.Image, out.Command, out.Status, out.Created)
		} else {
			fmt.Fprintln(w, out.Id)
		}
	}

	if !in.Quiet {
		w.Flush()
	}
	return nil
}

func cmdRestart(args []string) error {
	cmd := Subcmd("restart", "CONTAINER [CONTAINER...]", "Restart a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	body, err := apiPost("restart", cmd.Args())
	if err != nil {
		return err
	}

	var out []string
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	for _, name := range out {
		fmt.Println(name)
	}
	return nil
}

func cmdRm(args []string) error {
	cmd := Subcmd("rm", "CONTAINER [CONTAINER...]", "Remove a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	body, err := apiPost("rm", cmd.Args())
	if err != nil {
		return err
	}

	var out []string
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	for _, name := range out {
		fmt.Println(name)
	}
	return nil
}

func cmdRmi(args []string) error {
	cmd := Subcmd("rmi", "IMAGE [IMAGE...]", "Remove an image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	body, err := apiPost("rmi", cmd.Args())
	if err != nil {
		return err
	}

	var out []string
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	for _, name := range out {
		fmt.Println(name)
	}
	return nil
}

func cmdRun(args []string) error {
	config, err := ParseRun(args)
	if err != nil {
		return err
	}
	if config.Image == "" {
		fmt.Println("Error: Image not specified")
		return fmt.Errorf("Image not specified")
	}
	if len(config.Cmd) == 0 {
		fmt.Println("Error: Command not specified")
		return fmt.Errorf("Command not specified")
	}

	body, err := apiPostHijack("run", config)
	if err != nil {
		return err
	}
	defer body.Close()

	/*str, err2 := ioutil.ReadAll(body)
	if err2 != nil {
		return err2
	}
	fmt.Println(str)*/
	return nil

}

func cmdStart(args []string) error {
	cmd := Subcmd("start", "CONTAINER [CONTAINER...]", "Restart a stopped container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	body, err := apiPost("start", cmd.Args())
	if err != nil {
		return err
	}

	var out []string
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	for _, name := range out {
		fmt.Println(name)
	}

	return nil

}

func cmdStop(args []string) error {
	cmd := Subcmd("stop", "CONTAINER [CONTAINER...]", "Restart a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	body, err := apiPost("stop", cmd.Args())
	if err != nil {
		return err
	}

	var out []string
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	for _, name := range out {
		fmt.Println(name)
	}
	return nil

}

func cmdVersion(args []string) error {
	cmd := Subcmd("version", "", "Show the docker version information.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

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
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error: %s", body)
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
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("[error] %s", body)
	}
	return body, nil
}

func apiPostHijack(path string, data interface{}) (io.ReadCloser, error) {
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

	return resp.Body, nil
}

func Subcmd(name, signature, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Printf("\nUsage: docker %s %s\n\n%s\n\n", name, signature, description)
		flags.PrintDefaults()
	}
	return flags
}
