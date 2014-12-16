package main

import (
	"fmt"
	"os"
	"path"
	"syscall"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/cvfs"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/trust"
	"github.com/docker/docker/pkg/reexec"
)

const (
	defaultTrustKeyFile = "key.json"
	defaultCaFile       = "ca.pem"
	defaultKeyFile      = "key.pem"
	defaultCertFile     = "cert.pem"
)

func main() {
	if reexec.Init() {
		return
	}

	flag.Parse()
	
	if *flPurgeCache && *flReadOnly {
		log.Fatal("Cannot both mount readonly from cache and purge cache")
	}

	if *flVersion {
		showVersion()
		return
	}

	if *flLogLevel != "" {
		lvl, err := log.ParseLevel(*flLogLevel)
		if err != nil {
			log.Fatalf("Unable to parse logging level: %s", *flLogLevel)
		}
		initLogging(lvl)
	} else {
		initLogging(log.InfoLevel)
	}

	// -D, --debug, -l/--log-level=debug processing
	// When/if -D is removed this block can be deleted
	if *flDebug {
		os.Setenv("DEBUG", "1")
		initLogging(log.DebugLevel)
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	homeDir := os.Getenv("HOME")
	
	root := path.Join(homeDir, "pulldockercache")

	os.Setenv("DOCKER_DRIVER", "cvfs")
	driver, err := graphdriver.New(root, nil)
	if err != nil {
		log.Fatalf("Failed to start driver %s", err)
	}

	g, err := graph.NewGraph(root, driver)
	if err != nil {
		log.Fatal("Failed to start graph")
	}
	store, err := graph.NewTagStore(path.Join(root, "tags"), g, nil, nil)
	if err != nil {
		log.Fatal("Failed to start tag store")
	}

	trustDir := path.Join(root, "trust")
	if err = os.MkdirAll(trustDir, 0700); err != nil {
		log.Fatalf("Failed to setup trust store dir: %s", err)
	}
	trustStore, err := trust.NewTrustStore(trustDir); if err != nil {
		log.Fatal("Failed to start trust store")
	}

	eng := engine.New()
	store.Install(eng)
	trustStore.Install(eng)

	eng.Register("log", func(job *engine.Job) engine.Status {
		return engine.StatusOK
	})

	repotag := flag.Arg(0)
	repo, tag := parsers.ParseRepositoryTag(repotag)
	if tag == "" {
		tag = graph.DEFAULTTAG
	}
	pulljob := eng.Job("pull", repo, tag)
	store.CmdPull(pulljob)

	image, err := store.GetImage(repo, tag); if err != nil {
		log.Fatalf("Failed to get image: %s", err)
	}

	toplayer := path.Join(root, "cvfs", "dir", image.ID)


	dirname, err := filepath.Abs(*flOutputDir)
	if err != nil {
		log.Fatalf("Unknown output dir %s: %s", *flOutputDir, err)
	}

	if _, err := os.Stat(dirname); err == nil {
		// Directory already exists, use subfolder
		dirname = path.Join(dirname, repotag)
	} else {
		// Directory does not exist
		parent, _ := filepath.Split(dirname)
		if err = os.MkdirAll(parent, 0700); err != nil {
			log.Fatalf("Failed to setup output directory parents: %s", err)
		}
	}

	if *flReadOnly {
		// mount bind
		if err = os.Mkdir(dirname, 0700); err != nil {
			log.Fatalf("Failed to create output dir %s: %s", dirname, err)
		}
		// in order to mount readonly, 
		// you have to remount
		if err = syscall.Mount(toplayer, dirname, "", syscall.MS_BIND, ""); err != nil {
			log.Fatalf("Failed to mount flattened image: %s", err)
		}
		if err = syscall.Mount("", dirname, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
			log.Fatalf("Failed to mount read-only: %s", err)
		}
	} else if *flPurgeCache {
		// Just move the directory as an optimization
		// since we are purging the cache anyway
		if err = os.Rename(toplayer, dirname); err != nil {
			log.Fatalf("Failed to flatten image: %s", err)
		}
		if err = os.RemoveAll(root); err != nil {
			log.Fatalf("Failed to purge cache: %s", err)
		}
	} else {
		if err = archive.CopyWithTar(toplayer, dirname); err != nil {
			log.Fatalf("Copying image failed: %s", err)
		}
	}
}

func showVersion() {
	fmt.Printf("Pulldocker version %s, build %s\n", dockerversion.VERSION, dockerversion.GITCOMMIT)
}
