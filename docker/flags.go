package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

var (
	dockerCertPath  = os.Getenv("DOCKER_CERT_PATH")
	dockerTlsVerify = os.Getenv("DOCKER_TLS_VERIFY") != ""
)

func init() {
	if dockerCertPath == "" {
		dockerCertPath = filepath.Join(getHomeDir(), ".docker")
	}
}

func getHomeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

func getDaemonConfDir() string {
	// TODO: update for Windows daemon
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("USERPROFILE"), ".docker")
	}
	return "/etc/docker"
}

var (
	flVersion     = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
	flDaemon      = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
	flDebug       = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
	flSocketGroup = flag.String([]string{"G", "-group"}, "docker", "Group to assign the unix socket specified by -H when running in daemon mode\nuse '' (the empty string) to disable setting of a group")
	flLogLevel    = flag.String([]string{"l", "-log-level"}, "info", "Set the logging level (debug, info, warn, error, fatal)")
	flEnableCors  = flag.Bool([]string{"#api-enable-cors", "-api-enable-cors"}, false, "Enable CORS headers in the remote API")
	flTls         = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by --tlsverify flag")
	flHelp        = flag.Bool([]string{"h", "-help"}, false, "Print usage")
	flTlsVerify   = flag.Bool([]string{"-tlsverify"}, dockerTlsVerify, "Use TLS and verify the remote (daemon: verify client, client: verify daemon)")

	// these are initialized in init() below since their default values depend on dockerCertPath which isn't fully initialized until init() runs
	flTrustKey *string
	flCa       *string
	flCert     *string
	flKey      *string
	flHosts    []string
)

func setDefaultConfFlag(flag *string, def string) {
	if *flag == "" {
		if *flDaemon {
			*flag = filepath.Join(getDaemonConfDir(), def)
		} else {
			*flag = filepath.Join(getHomeDir(), ".docker", def)
		}
	}
}

func init() {
	var placeholderTrustKey string
	// TODO use flag flag.String([]string{"i", "-identity"}, "", "Path to libtrust key file")
	flTrustKey = &placeholderTrustKey

	flCa = flag.String([]string{"-tlscacert"}, filepath.Join(dockerCertPath, defaultCaFile), "Trust only remotes providing a certificate signed by the CA given here")
	flCert = flag.String([]string{"-tlscert"}, filepath.Join(dockerCertPath, defaultCertFile), "Path to TLS certificate file")
	flKey = flag.String([]string{"-tlskey"}, filepath.Join(dockerCertPath, defaultKeyFile), "Path to TLS key file")
	opts.HostListVar(&flHosts, []string{"H", "-host"}, "The socket(s) to bind to in daemon mode or connect to in client mode, specified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.")

	flag.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: docker [OPTIONS] COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nOptions:\n")

		flag.CommandLine.SetOutput(os.Stdout)
		flag.PrintDefaults()

		help := "\nCommands:\n"

		for _, command := range [][]string{
			{"attach", utils.CmdDescAttach},
			{"build", utils.CmdDescBuild},
			{"commit", utils.CmdDescCommit},
			{"cp", utils.CmdDescCp},
			{"create", utils.CmdDescCreate},
			{"diff", utils.CmdDescDiff},
			{"events", utils.CmdDescEvents},
			{"exec", utils.CmdDescExec},
			{"export", utils.CmdDescExport},
			{"history", utils.CmdDescHistory},
			{"images", utils.CmdDescImages},
			{"import", utils.CmdDescImport},
			{"info", utils.CmdDescInfo},
			{"inspect", utils.CmdDescInspect},
			{"kill", utils.CmdDescKill},
			{"load", utils.CmdDescLoad},
			{"login", utils.CmdDescLogin},
			{"logout", utils.CmdDescLogout},
			{"logs", utils.CmdDescLogs},
			{"port", utils.CmdDescPort},
			{"pause", utils.CmdDescPause},
			{"ps", utils.CmdDescPs},
			{"pull", utils.CmdDescPull},
			{"push", utils.CmdDescPush},
			{"rename", utils.CmdDescRename},
			{"restart", utils.CmdDescRestart},
			{"rm", utils.CmdDescRm},
			{"rmi", utils.CmdDescRmi},
			{"run", utils.CmdDescRun},
			{"save", utils.CmdDescSave},
			{"search", utils.CmdDescSearch},
			{"start", utils.CmdDescStart},
			{"stats", utils.CmdDescStats},
			{"stop", utils.CmdDescStop},
			{"tag", utils.CmdDescTag},
			{"top", utils.CmdDescTop},
			{"unpause", utils.CmdDescUnpause},
			{"version", utils.CmdDescVersion},
			{"wait", utils.CmdDescWait},
		} {
			help += fmt.Sprintf("    %-10.10s%s\n", command[0], command[1])
		}
		help += "\nRun 'docker COMMAND --help' for more information on a command."
		fmt.Fprintf(os.Stdout, "%s\n", help)
	}
}
