// +build daemon

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builtins"
	"github.com/docker/docker/daemon"
	_ "github.com/docker/docker/daemon/execdriver/lxc"
	_ "github.com/docker/docker/daemon/execdriver/native"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/homedir"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/timeutils"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

const CanDaemon = true

var (
	daemonCfg   = &daemon.Config{}
	registryCfg = &registry.Options{}
)

func init() {
	daemonCfg.InstallFlags()
	registryCfg.InstallFlags()
}

func migrateKey() (err error) {
	// Migrate trust key if exists at ~/.docker/key.json and owned by current user
	oldPath := filepath.Join(homedir.Get(), ".docker", defaultTrustKeyFile)
	newPath := filepath.Join(getDaemonConfDir(), defaultTrustKeyFile)
	if _, statErr := os.Stat(newPath); os.IsNotExist(statErr) && utils.IsFileOwner(oldPath) {
		defer func() {
			// Ensure old path is removed if no error occurred
			if err == nil {
				err = os.Remove(oldPath)
			} else {
				logrus.Warnf("Key migration failed, key file not removed at %s", oldPath)
			}
		}()

		if err := os.MkdirAll(getDaemonConfDir(), os.FileMode(0644)); err != nil {
			return fmt.Errorf("Unable to create daemon configuration directory: %s", err)
		}

		newFile, err := os.OpenFile(newPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("error creating key file %q: %s", newPath, err)
		}
		defer newFile.Close()

		oldFile, err := os.Open(oldPath)
		if err != nil {
			return fmt.Errorf("error opening key file %q: %s", oldPath, err)
		}
		defer oldFile.Close()

		if _, err := io.Copy(newFile, oldFile); err != nil {
			return fmt.Errorf("error copying key: %s", err)
		}

		logrus.Infof("Migrated key from %s to %s", oldPath, newPath)
	}

	return nil
}

func mainDaemon() {
	if flag.NArg() != 0 {
		flag.Usage()
		return
	}

	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: timeutils.RFC3339NanoFixed})

	eng := engine.New()
	signal.Trap(eng.Shutdown)

	if err := migrateKey(); err != nil {
		logrus.Fatal(err)
	}
	daemonCfg.TrustKeyPath = *flTrustKey

	// Load builtins
	if err := builtins.Register(eng); err != nil {
		logrus.Fatal(err)
	}

	registryService := registry.NewService(registryCfg)
	// load the daemon in the background so we can immediately start
	// the http api so that connections don't fail while the daemon
	// is booting
	daemonInitWait := make(chan error)
	go func() {
		d, err := daemon.NewDaemon(daemonCfg, eng, registryService)
		if err != nil {
			daemonInitWait <- err
			return
		}

		logrus.WithFields(logrus.Fields{
			"version":     dockerversion.VERSION,
			"commit":      dockerversion.GITCOMMIT,
			"execdriver":  d.ExecutionDriver().Name(),
			"graphdriver": d.GraphDriver().String(),
		}).Info("Docker daemon")

		if err := d.Install(eng); err != nil {
			daemonInitWait <- err
			return
		}

		b := &builder.BuilderJob{eng, d}
		b.Install()

		// after the daemon is done setting up we can tell the api to start
		// accepting connections
		if err := eng.Job("acceptconnections").Run(); err != nil {
			daemonInitWait <- err
			return
		}
		daemonInitWait <- nil
	}()

	// Serve api
	job := eng.Job("serveapi", flHosts...)
	job.SetenvBool("Logging", true)
	job.SetenvBool("EnableCors", daemonCfg.EnableCors)
	job.Setenv("CorsHeaders", daemonCfg.CorsHeaders)
	job.Setenv("Version", dockerversion.VERSION)
	job.Setenv("SocketGroup", daemonCfg.SocketGroup)

	job.SetenvBool("Tls", *flTls)
	job.SetenvBool("TlsVerify", *flTlsVerify)
	job.Setenv("TlsCa", *flCa)
	job.Setenv("TlsCert", *flCert)
	job.Setenv("TlsKey", *flKey)

	// The serve API job never exits unless an error occurs
	// We need to start it as a goroutine and wait on it so
	// daemon doesn't exit
	serveAPIWait := make(chan error)
	go func() {
		if err := job.Run(); err != nil {
			logrus.Errorf("ServeAPI error: %v", err)
			serveAPIWait <- err
			return
		}
		serveAPIWait <- nil
	}()

	// Wait for the daemon startup goroutine to finish
	// This makes sure we can actually cleanly shutdown the daemon
	logrus.Debug("waiting for daemon to initialize")
	errDaemon := <-daemonInitWait
	if errDaemon != nil {
		eng.Shutdown()
		outStr := fmt.Sprintf("Shutting down daemon due to errors: %v", errDaemon)
		if strings.Contains(errDaemon.Error(), "engine is shutdown") {
			// if the error is "engine is shutdown", we've already reported (or
			// will report below in API server errors) the error
			outStr = "Shutting down daemon due to reported errors"
		}
		// we must "fatal" exit here as the API server may be happy to
		// continue listening forever if the error had no impact to API
		logrus.Fatal(outStr)
	} else {
		logrus.Info("Daemon has completed initialization")
	}

	// Daemon is fully initialized and handling API traffic
	// Wait for serve API job to complete
	errAPI := <-serveAPIWait
	// If we have an error here it is unique to API (as daemonErr would have
	// exited the daemon process above)
	eng.Shutdown()
	if errAPI != nil {
		logrus.Fatalf("Shutting down due to ServeAPI error: %v", errAPI)
	}

}
