package main

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/cmd/networkdb-test/dbclient"
	"github.com/docker/docker/libnetwork/cmd/networkdb-test/dbserver"
)

func main() {
	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}
	logrus.SetFormatter(formatter)
	log.G(context.TODO()).Infof("Starting the image with these args: %v", os.Args)
	if len(os.Args) < 1 {
		log.G(context.TODO()).Fatal("You need at least 1 argument [client/server]")
	}

	switch os.Args[1] {
	case "server":
		dbserver.Server(os.Args[2:])
	case "client":
		dbclient.Client(os.Args[2:])
	}
}
