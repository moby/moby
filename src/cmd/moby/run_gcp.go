package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
)

const (
	defaultZone     = "europe-west1-d"
	defaultMachine  = "g1-small"
	defaultDiskSize = 1
	zoneVar         = "MOBY_GCP_ZONE"
	machineVar      = "MOBY_GCP_MACHINE"
	keysVar         = "MOBY_GCP_KEYS"
	projectVar      = "MOBY_GCP_PROJECT"
	bucketVar       = "MOBY_GCP_BUCKET"
	familyVar       = "MOBY_GCP_FAMILY"
	publicVar       = "MOBY_GCP_PUBLIC"
	nameVar         = "MOBY_GCP_IMAGE_NAME"
	diskSizeVar     = "MOBY_GCP_DISK_SIZE"
)

// Process the run arguments and execute run
func runGcp(args []string) {
	gcpCmd := flag.NewFlagSet("gcp", flag.ExitOnError)
	gcpCmd.Usage = func() {
		fmt.Printf("USAGE: %s run gcp [options] [name]\n\n", os.Args[0])
		fmt.Printf("'name' specifies either the name of an already uploaded\n")
		fmt.Printf("GCP image or the full path to a image file which will be\n")
		fmt.Printf("uploaded before it is run.\n\n")
		fmt.Printf("Options:\n\n")
		gcpCmd.PrintDefaults()
	}
	zoneFlag := gcpCmd.String("zone", defaultZone, "GCP Zone")
	machineFlag := gcpCmd.String("machine", defaultMachine, "GCP Machine Type")
	keysFlag := gcpCmd.String("keys", "", "Path to Service Account JSON key file")
	projectFlag := gcpCmd.String("project", "", "GCP Project Name")
	bucketFlag := gcpCmd.String("bucket", "", "GS Bucket to upload to. *Required* when 'prefix' is a filename")
	publicFlag := gcpCmd.Bool("public", false, "Select if file on GS should be public. *Optional* when 'prefix' is a filename")
	familyFlag := gcpCmd.String("family", "", "GCP Image Family. A group of images where the family name points to the most recent image. *Optional* when 'prefix' is a filename")
	nameFlag := gcpCmd.String("img-name", "", "Overrides the Name used to identify the file in Google Storage, Image and Instance. Defaults to [name]")
	diskSizeFlag := gcpCmd.Int("disk-size", 0, "Size of system disk in GB")

	if err := gcpCmd.Parse(args); err != nil {
		log.Fatal("Unable to parse args")
	}

	remArgs := gcpCmd.Args()
	if len(remArgs) == 0 {
		fmt.Printf("Please specify the prefix to the image to boot\n")
		gcpCmd.Usage()
		os.Exit(1)
	}
	prefix := remArgs[0]

	zone := getStringValue(zoneVar, *zoneFlag, defaultZone)
	machine := getStringValue(machineVar, *machineFlag, defaultMachine)
	keys := getStringValue(keysVar, *keysFlag, "")
	project := getStringValue(projectVar, *projectFlag, "")
	bucket := getStringValue(bucketVar, *bucketFlag, "")
	public := getBoolValue(publicVar, *publicFlag)
	family := getStringValue(familyVar, *familyFlag, "")
	name := getStringValue(nameVar, *nameFlag, "")
	diskSize := getIntValue(diskSizeVar, *diskSizeFlag, defaultDiskSize)

	client, err := NewGCPClient(keys, project)
	if err != nil {
		log.Fatalf("Unable to connect to GCP")
	}

	suffix := ".img.tar.gz"
	if strings.HasSuffix(prefix, suffix) {
		src := prefix
		if name != "" {
			prefix = name
		} else {
			prefix = prefix[:len(prefix)-len(suffix)]
		}
		if bucket == "" {
			log.Fatalf("No bucket specified. Please provide one using the -bucket flag")
		}
		err = client.UploadFile(src, prefix+suffix, bucket, public)
		if err != nil {
			log.Fatalf("Error copying to Google Storage: %v", err)
		}
		err = client.CreateImage(prefix, "https://storage.googleapis.com/"+bucket+"/"+prefix+".img.tar.gz", family, true)
		if err != nil {
			log.Fatalf("Error creating Google Compute Image: %v", err)
		}
	}

	// If no name was supplied, use the prefix
	if name == "" {
		name = prefix
	}

	if err = client.CreateInstance(name, prefix, zone, machine, diskSize, true); err != nil {
		log.Fatal(err)
	}

	if err = client.ConnectToInstanceSerialPort(name, zone); err != nil {
		log.Fatal(err)
	}

	if err = client.DeleteInstance(name, zone, true); err != nil {
		log.Fatal(err)
	}
}
