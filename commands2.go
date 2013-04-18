package docker

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"encoding/json"
)

func ParseCommands(args []string) error {
	
	cmds := map[string]func(args []string) error {
		"version":cmdVersion,
	}

	if len(args) > 0 {
		cmd, exists :=  cmds[args[0]]
		if !exists {
			//TODO display commend not found
			return cmdHelp(args)
		}
		return cmd(args)
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
//		{"images", "List images"},
//		{"import", "Create a new filesystem image from the contents of a tarball"},
//		{"info", "Display system-wide information"},
//		{"inspect", "Return low-level information on a container"},
//		{"kill", "Kill a running container"},
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

func cmdVersion(args []string) error {
	body, err := apiCall("version")
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

func apiCall(path string) ([]byte, error) {
	resp, err := http.Get("http://0.0.0.0:4243/" + path) 
        if err != nil {
                return nil,err
        }
	//TODO check status code
        defer resp.Body.Close()
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                return nil, err
        }
	return body, nil

}