package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/cgroups/systemd"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "Create a cgroup container using the supplied configuration and initial process.",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "config, c", Value: "cgroup.json", Usage: "path to container configuration (cgroups.Cgroup object)"},
		cli.IntFlag{Name: "pid, p", Value: 0, Usage: "pid of the initial process in the container"},
	},
	Action: createAction,
}

var destroyCommand = cli.Command{
	Name:  "destroy",
	Usage: "Destroy an existing cgroup container.",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "name, n", Value: "", Usage: "container name"},
		cli.StringFlag{Name: "parent, p", Value: "", Usage: "container parent"},
	},
	Action: destroyAction,
}

var statsCommand = cli.Command{
	Name:  "stats",
	Usage: "Get stats for cgroup",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "name, n", Value: "", Usage: "container name"},
		cli.StringFlag{Name: "parent, p", Value: "", Usage: "container parent"},
	},
	Action: statsAction,
}

var pauseCommand = cli.Command{
	Name:  "pause",
	Usage: "Pause cgroup",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "name, n", Value: "", Usage: "container name"},
		cli.StringFlag{Name: "parent, p", Value: "", Usage: "container parent"},
	},
	Action: pauseAction,
}

var resumeCommand = cli.Command{
	Name:  "resume",
	Usage: "Resume a paused cgroup",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "name, n", Value: "", Usage: "container name"},
		cli.StringFlag{Name: "parent, p", Value: "", Usage: "container parent"},
	},
	Action: resumeAction,
}

var psCommand = cli.Command{
	Name:  "ps",
	Usage: "Get list of pids for a cgroup",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "name, n", Value: "", Usage: "container name"},
		cli.StringFlag{Name: "parent, p", Value: "", Usage: "container parent"},
	},
	Action: psAction,
}

func getConfigFromFile(c *cli.Context) (*cgroups.Cgroup, error) {
	f, err := os.Open(c.String("config"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config *cgroups.Cgroup
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		log.Fatal(err)
	}
	return config, nil
}

func openLog(name string) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0755)
	if err != nil {
		return err
	}

	log.SetOutput(f)
	return nil
}

func getConfig(context *cli.Context) (*cgroups.Cgroup, error) {
	name := context.String("name")
	if name == "" {
		log.Fatal(fmt.Errorf("Missing container name"))
	}
	parent := context.String("parent")
	return &cgroups.Cgroup{
		Name:   name,
		Parent: parent,
	}, nil
}

func killAll(config *cgroups.Cgroup) {
	// We could use freezer here to prevent process spawning while we are trying
	// to kill everything. But going with more portable solution of retrying for
	// now.
	pids := getPids(config)
	retry := 10
	for len(pids) != 0 || retry > 0 {
		killPids(pids)
		time.Sleep(100 * time.Millisecond)
		retry--
		pids = getPids(config)
	}
	if len(pids) != 0 {
		log.Fatal(fmt.Errorf("Could not kill existing processes in the container."))
	}
}

func getPids(config *cgroups.Cgroup) []int {
	pids, err := fs.GetPids(config)
	if err != nil {
		log.Fatal(err)
	}
	return pids
}

func killPids(pids []int) {
	for _, pid := range pids {
		// pids might go away on their own. Ignore errors.
		syscall.Kill(pid, syscall.SIGKILL)
	}
}

func setFreezerState(context *cli.Context, state cgroups.FreezerState) {
	config, err := getConfig(context)
	if err != nil {
		log.Fatal(err)
	}

	if systemd.UseSystemd() {
		err = systemd.Freeze(config, state)
	} else {
		err = fs.Freeze(config, state)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func createAction(context *cli.Context) {
	config, err := getConfigFromFile(context)
	if err != nil {
		log.Fatal(err)
	}
	pid := context.Int("pid")
	if pid <= 0 {
		log.Fatal(fmt.Errorf("Invalid pid : %d", pid))
	}
	if systemd.UseSystemd() {
		_, err := systemd.Apply(config, pid)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		_, err := fs.Apply(config, pid)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func destroyAction(context *cli.Context) {
	config, err := getConfig(context)
	if err != nil {
		log.Fatal(err)
	}

	killAll(config)
	// Systemd will clean up cgroup state for empty container.
	if !systemd.UseSystemd() {
		err := fs.Cleanup(config)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func statsAction(context *cli.Context) {
	config, err := getConfig(context)
	if err != nil {
		log.Fatal(err)
	}
	stats, err := fs.GetStats(config)
	if err != nil {
		log.Fatal(err)
	}

	out, err := json.MarshalIndent(stats, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Usage stats for '%s':\n %v\n", config.Name, string(out))
}

func pauseAction(context *cli.Context) {
	setFreezerState(context, cgroups.Frozen)
}

func resumeAction(context *cli.Context) {
	setFreezerState(context, cgroups.Thawed)
}

func psAction(context *cli.Context) {
	config, err := getConfig(context)
	if err != nil {
		log.Fatal(err)
	}

	pids, err := fs.GetPids(config)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Pids in '%s':\n", config.Name)
	fmt.Println(pids)
}

func main() {
	logPath := os.Getenv("log")
	if logPath != "" {
		if err := openLog(logPath); err != nil {
			log.Fatal(err)
		}
	}

	app := cli.NewApp()
	app.Name = "cgutil"
	app.Usage = "Test utility for libcontainer cgroups package"
	app.Version = "0.1"

	app.Commands = []cli.Command{
		createCommand,
		destroyCommand,
		statsCommand,
		pauseCommand,
		resumeCommand,
		psCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
