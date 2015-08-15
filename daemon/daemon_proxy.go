// +build linux

package daemon

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/proxy"
	"github.com/docker/docker/pkg/mflag"
	"net"
	"net/http"
	"net/rpc"
	"strings"
)

type ProxyConfig struct {
	Root      string
	CtName    string
	ProtoAddr string
}

func (config *ProxyConfig) InstallProxyFlags(cmd *mflag.FlagSet, usageFn func(string) string) {
	cmd.StringVar(&config.Root, []string{"R", "-root"}, defaultGraph, usageFn("Root of container runtime"))
	cmd.StringVar(&config.CtName, []string{"C", "-ctname"}, "", usageFn("Container we work for"))
	cmd.StringVar(&config.ProtoAddr, []string{"S", "-sockname"}, "", usageFn("Socket to listen"))
}

func StartProxyDaemon(config *ProxyConfig, cli *cli.Cli) {
	p := new(graphdriver.ProxyAPI)
	p.Root = config.Root
	p.CtName = config.CtName
	p.Cli = cli

	if p.CtName == "" || config.ProtoAddr == "" {
		logrus.Fatal("set both \"ctname\" and \"sockname\"")
	}

	rpc.Register(p)
	rpc.HandleHTTP()
	protoAddrParts := strings.SplitN(config.ProtoAddr, "://", 2)
	l, e := net.Listen(protoAddrParts[0], protoAddrParts[1])
	if e != nil {
		logrus.Fatal("listen error:", e)
	}
	http.Serve(l, nil)
}
