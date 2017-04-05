package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
)

// Process the run arguments and execute run
func runGcp(args []string) {
	gcpCmd := flag.NewFlagSet("gcp", flag.ExitOnError)
	gcpCmd.Usage = func() {
		fmt.Printf("USAGE: %s run gcp [options] [name]\n\n", os.Args[0])
		fmt.Printf("'name' specifies either the name of an already uploaded\n")
		fmt.Printf("GCE image or the full path to a image file which will be\n")
		fmt.Printf("uploaded before it is run.\n\n")
		fmt.Printf("Options:\n\n")
		gcpCmd.PrintDefaults()
	}
	zone := gcpCmd.String("zone", "europe-west1-d", "GCP Zone")
	machine := gcpCmd.String("machine", "g1-small", "GCE Machine Type")
	keys := gcpCmd.String("keys", "", "Path to Service Account JSON key file")
	project := gcpCmd.String("project", "", "GCP Project Name")
	bucket := gcpCmd.String("bucket", "", "GS Bucket to upload to. *Required* when 'prefix' is a filename")
	public := gcpCmd.Bool("public", false, "Select if file on GS should be public. *Optional* when 'prefix' is a filename")
	family := gcpCmd.String("family", "", "GCE Image Family. A group of images where the family name points to the most recent image. *Optional* when 'prefix' is a filename")

	gcpCmd.Parse(args)
	remArgs := gcpCmd.Args()
	if len(remArgs) == 0 {
		fmt.Printf("Please specify the prefix to the image to boot\n")
		gcpCmd.Usage()
		os.Exit(1)
	}
	prefix := remArgs[0]

	client, err := NewGCPClient(*keys, *project)
	if err != nil {
		log.Fatalf("Unable to connect to GCP")
	}

	suffix := ".img.tar.gz"
	if strings.HasSuffix(prefix, suffix) {
		filename := prefix
		prefix = prefix[:len(prefix)-len(suffix)]
		if *bucket == "" {
			log.Fatalf("No bucket specified. Please provide one using the -bucket flag")
		}
		err = client.UploadFile(filename, *bucket, *public)
		if err != nil {
			log.Fatalf("Error copying to Google Storage: %v", err)
		}
		err = client.CreateImage(prefix, "https://storage.googleapis.com/"+*bucket+"/"+prefix+".img.tar.gz", *family, true)
		if err != nil {
			log.Fatalf("Error creating Google Compute Image: %v", err)
		}
	}

	if err = client.CreateInstance(prefix, *zone, *machine, true); err != nil {
		log.Fatal(err)
	}

	if err = client.ConnectToInstanceSerialPort(prefix, *zone); err != nil {
		log.Fatal(err)
	}

	if err = client.DeleteInstance(prefix, *zone, true); err != nil {
		log.Fatal(err)
	}
}
