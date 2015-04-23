package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/registry"
)

func (cli *DockerCli) CmdUpdate(args ...string) error {
	cmd := cli.Subcmd("update", "[REPOSITORY]", "Update images", true)
	dryRun := cmd.Bool([]string{"d", "-dry-run"}, false, "Only show out of date images")

	cmd.Require(flag.Max, 1)

	cmd.ParseFlags(args, true)

	v := url.Values{}
	if *dryRun {
		v.Set("dry_run", "1")
	}

	authConfigs := cli.configFile.AuthConfigs
	update := func(authConfigs map[string]registry.AuthConfig) error {
		buf, err := json.Marshal(authConfigs)
		if err != nil {
			return err
		}
		registryAuthHeader := []string{
			base64.URLEncoding.EncodeToString(buf),
		}
		return cli.stream("POST", "/images/update?"+v.Encode(), nil, cli.out, map[string][]string{
			"X-Registry-Auth": registryAuthHeader,
		})
	}

	if err := update(authConfigs); err != nil {
		if strings.Contains(err.Error(), "Status 401") {
			fmt.Fprintln(cli.out, "\nAt least one repository requires login before updating")
		}
		return err
	}
	return nil
}
