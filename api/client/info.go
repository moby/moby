package client

import (
	"fmt"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/units"
	"github.com/docker/docker/utils"
)

// CmdInfo displays system-wide information.
//
// Usage: docker info
func (cli *DockerCli) CmdInfo(args ...string) error {
	cmd := cli.Subcmd("info", "", "Display system-wide information", true)
	cmd.Require(flag.Exact, 0)
	utils.ParseFlags(cmd, args, false)

	body, _, err := readBody(cli.call("GET", "/info", nil, false))
	if err != nil {
		return err
	}

	out := engine.NewOutput()
	remoteInfo, err := out.AddEnv()
	if err != nil {
		return err
	}

	if _, err := out.Write(body); err != nil {
		log.Errorf("Error reading remote info: %s", err)
		return err
	}
	out.Close()

	if remoteInfo.Exists("Containers") {
		fmt.Fprintf(cli.out, "Containers: %d\n", remoteInfo.GetInt("Containers"))
	}
	if remoteInfo.Exists("Images") {
		fmt.Fprintf(cli.out, "Images: %d\n", remoteInfo.GetInt("Images"))
	}
	if remoteInfo.Exists("Driver") {
		fmt.Fprintf(cli.out, "Storage Driver: %s\n", remoteInfo.Get("Driver"))
	}
	if remoteInfo.Exists("DriverStatus") {
		var driverStatus [][2]string
		if err := remoteInfo.GetJson("DriverStatus", &driverStatus); err != nil {
			return err
		}
		for _, pair := range driverStatus {
			fmt.Fprintf(cli.out, " %s: %s\n", pair[0], pair[1])
		}
	}
	if remoteInfo.Exists("ExecutionDriver") {
		fmt.Fprintf(cli.out, "Execution Driver: %s\n", remoteInfo.Get("ExecutionDriver"))
	}
	if remoteInfo.Exists("KernelVersion") {
		fmt.Fprintf(cli.out, "Kernel Version: %s\n", remoteInfo.Get("KernelVersion"))
	}
	if remoteInfo.Exists("OperatingSystem") {
		fmt.Fprintf(cli.out, "Operating System: %s\n", remoteInfo.Get("OperatingSystem"))
	}
	if remoteInfo.Exists("NCPU") {
		fmt.Fprintf(cli.out, "CPUs: %d\n", remoteInfo.GetInt("NCPU"))
	}
	if remoteInfo.Exists("MemTotal") {
		fmt.Fprintf(cli.out, "Total Memory: %s\n", units.BytesSize(float64(remoteInfo.GetInt64("MemTotal"))))
	}
	if remoteInfo.Exists("Name") {
		fmt.Fprintf(cli.out, "Name: %s\n", remoteInfo.Get("Name"))
	}
	if remoteInfo.Exists("ID") {
		fmt.Fprintf(cli.out, "ID: %s\n", remoteInfo.Get("ID"))
	}

	if remoteInfo.GetBool("Debug") || os.Getenv("DEBUG") != "" {
		if remoteInfo.Exists("Debug") {
			fmt.Fprintf(cli.out, "Debug mode (server): %v\n", remoteInfo.GetBool("Debug"))
		}
		fmt.Fprintf(cli.out, "Debug mode (client): %v\n", os.Getenv("DEBUG") != "")
		if remoteInfo.Exists("NFd") {
			fmt.Fprintf(cli.out, "File Descriptors: %d\n", remoteInfo.GetInt("NFd"))
		}
		if remoteInfo.Exists("NGoroutines") {
			fmt.Fprintf(cli.out, "Goroutines: %d\n", remoteInfo.GetInt("NGoroutines"))
		}
		if remoteInfo.Exists("SystemTime") {
			t, err := remoteInfo.GetTime("SystemTime")
			if err != nil {
				log.Errorf("Error reading system time: %v", err)
			} else {
				fmt.Fprintf(cli.out, "System Time: %s\n", t.Format(time.UnixDate))
			}
		}
		if remoteInfo.Exists("NEventsListener") {
			fmt.Fprintf(cli.out, "EventsListeners: %d\n", remoteInfo.GetInt("NEventsListener"))
		}
		if initSha1 := remoteInfo.Get("InitSha1"); initSha1 != "" {
			fmt.Fprintf(cli.out, "Init SHA1: %s\n", initSha1)
		}
		if initPath := remoteInfo.Get("InitPath"); initPath != "" {
			fmt.Fprintf(cli.out, "Init Path: %s\n", initPath)
		}
		if root := remoteInfo.Get("DockerRootDir"); root != "" {
			fmt.Fprintf(cli.out, "Docker Root Dir: %s\n", root)
		}
	}
	if remoteInfo.Exists("HttpProxy") {
		fmt.Fprintf(cli.out, "Http Proxy: %s\n", remoteInfo.Get("HttpProxy"))
	}
	if remoteInfo.Exists("HttpsProxy") {
		fmt.Fprintf(cli.out, "Https Proxy: %s\n", remoteInfo.Get("HttpsProxy"))
	}
	if remoteInfo.Exists("NoProxy") {
		fmt.Fprintf(cli.out, "No Proxy: %s\n", remoteInfo.Get("NoProxy"))
	}
	if len(remoteInfo.GetList("IndexServerAddress")) != 0 {
		cli.LoadConfigFile()
		u := cli.configFile.Configs[remoteInfo.Get("IndexServerAddress")].Username
		if len(u) > 0 {
			fmt.Fprintf(cli.out, "Username: %v\n", u)
			fmt.Fprintf(cli.out, "Registry: %v\n", remoteInfo.GetList("IndexServerAddress"))
		}
	}
	if remoteInfo.Exists("MemoryLimit") && !remoteInfo.GetBool("MemoryLimit") {
		fmt.Fprintf(cli.err, "WARNING: No memory limit support\n")
	}
	if remoteInfo.Exists("SwapLimit") && !remoteInfo.GetBool("SwapLimit") {
		fmt.Fprintf(cli.err, "WARNING: No swap limit support\n")
	}
	if remoteInfo.Exists("IPv4Forwarding") && !remoteInfo.GetBool("IPv4Forwarding") {
		fmt.Fprintf(cli.err, "WARNING: IPv4 forwarding is disabled.\n")
	}
	if remoteInfo.Exists("Labels") {
		fmt.Fprintln(cli.out, "Labels:")
		for _, attribute := range remoteInfo.GetList("Labels") {
			fmt.Fprintf(cli.out, " %s\n", attribute)
		}
	}

	return nil
}
