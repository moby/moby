package client

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/Sirupsen/logrus"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdStop stops one or more running containers.
//
// A running container is stopped by first sending SIGTERM and then SIGKILL if
// the container fails to stop within a grace period (the default is 10 seconds).
// If --rm flag is supplied the container will be deleted along with any volume
// associated to it when it stops.
//
// Usage: docker stop [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdStop(args ...string) error {
	cmd := cli.Subcmd("stop", "CONTAINER [CONTAINER...]", "Stop a running container by sending SIGTERM and then SIGKILL after a\ngrace period", true)
	flSeconds := cmd.Int([]string{"t", "-time"}, 10, "Seconds to wait for stop before killing it")
	flRemove := cmd.Bool([]string{"#rm", "-rm"}, false, "Automatically remove the container and the volumes associated to it when it stops")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	v := url.Values{}
	v.Set("t", strconv.Itoa(*flSeconds))

	var encounteredError error
	for _, name := range cmd.Args() {
		_, _, err := readBody(cli.call("POST", "/containers/"+name+"/stop?"+v.Encode(), nil, nil))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to stop one or more containers")
		} else {
			if *flRemove {
				if _, _, err = readBody(cli.call("DELETE", "/containers/"+name+"?v=1", nil, nil)); err != nil {
					logrus.Errorf("Error deleting container: %s", err)
				}
			}
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}
