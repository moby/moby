package main

import (
	"encoding/json"
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/packethost/packngo"
	"net/http"
	"os"
)

const (
	packetDefaultZone     = "ams1"
	packetDefaultMachine  = "baremetal_0"
	packetDefaultHostname = "moby"
	packetBaseURL         = "PACKET_BASE_URL"
	packetZoneVar         = "PACKET_ZONE"
	packetMachineVar      = "PACKET_MACHINE"
	packetAPIKeyVar       = "PACKET_API_KEY"
	packetProjectIDVar    = "PACKET_PROJECT_ID"
	packetHostnameVar     = "PACKET_HOSTNAME"
	packetNameVar         = "PACKET_NAME"
)

// ValidateHTTPURL does a sanity check that a URL returns a 200 or 300 response
func ValidateHTTPURL(url string) {
	log.Printf("Validating URL: %s", url)
	resp, err := http.Head(url)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode >= 400 {
		log.Fatal("Got a non 200- or 300- HTTP response code: %s", resp)
	}
	log.Printf("OK: %d response code", resp.StatusCode)
}

// Process the run arguments and execute run
func runPacket(args []string) {
	packetCmd := flag.NewFlagSet("packet", flag.ExitOnError)
	packetCmd.Usage = func() {
		fmt.Printf("USAGE: %s run packet [options] [name]\n\n", os.Args[0])
		fmt.Printf("Options:\n\n")
		packetCmd.PrintDefaults()
	}
	baseURLFlag := packetCmd.String("base-url", "", "Base URL that the kernel and initrd are served from.")
	zoneFlag := packetCmd.String("zone", packetDefaultZone, "Packet Zone")
	machineFlag := packetCmd.String("machine", packetDefaultMachine, "Packet Machine Type")
	apiKeyFlag := packetCmd.String("api-key", "", "Packet API key")
	projectFlag := packetCmd.String("project-id", "", "Packet Project ID")
	hostNameFlag := packetCmd.String("hostname", packetDefaultHostname, "Hostname of new instance")
	nameFlag := packetCmd.String("img-name", "", "Overrides the prefix used to identify the files. Defaults to [name]")
	if err := packetCmd.Parse(args); err != nil {
		log.Fatal("Unable to parse args")
	}

	remArgs := packetCmd.Args()
	prefix := "packet"
	if len(remArgs) > 0 {
		prefix = remArgs[0]
	}

	url := getStringValue(packetBaseURL, *baseURLFlag, "")
	if url == "" {
		log.Fatal("Need to specify a value for --base-url where the images are hosted. This URL should contain <url>/%s-bzImage and <url>/%s-initrd.img")
	}
	facility := getStringValue(packetZoneVar, *zoneFlag, "")
	plan := getStringValue(packetMachineVar, *machineFlag, defaultMachine)
	apiKey := getStringValue(packetAPIKeyVar, *apiKeyFlag, "")
	if apiKey == "" {
		log.Fatal("Must specify a Packet.net API key with --api-key")
	}
	projectID := getStringValue(packetProjectIDVar, *projectFlag, "")
	if projectID == "" {
		log.Fatal("Must specify a Packet.net Project ID with --project-id")
	}
	hostname := getStringValue(packetHostnameVar, *hostNameFlag, "")
	name := getStringValue(packetNameVar, *nameFlag, prefix)
	osType := "custom_ipxe"
	billing := "hourly"
	userData := fmt.Sprintf("#!ipxe\n\ndhcp\nset base-url %s\nset kernel-params ip=dhcp nomodeset ro serial console=ttyS1,115200\nkernel ${base-url}/%s-bzImage ${kernel-params}\ninitrd ${base-url}/%s-initrd.img\nboot", url, name, name)
	log.Debugf("Using userData of:\n%s\n", userData)
	initrdURL := fmt.Sprintf("%s/%s-initrd.img", url, name)
	kernelURL := fmt.Sprintf("%s/%s-bzImage", url, name)
	ValidateHTTPURL(kernelURL)
	ValidateHTTPURL(initrdURL)
	client := packngo.NewClient("", apiKey, nil)
	tags := []string{}
	req := packngo.DeviceCreateRequest{
		HostName:     hostname,
		Plan:         plan,
		Facility:     facility,
		OS:           osType,
		BillingCycle: billing,
		ProjectID:    projectID,
		UserData:     userData,
		Tags:         tags,
	}
	d, _, err := client.Devices.Create(&req)
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.MarshalIndent(d, "", "    ")
	if err != nil {
		log.Fatal(err)
	}
	// log response json if in verbose mode
	log.Debugf("%s\n", string(b))
	// TODO: poll events api for bringup (requires extpacknogo)
	// TODO: connect to serial console (requires API extension to get SSH URI)
	// TODO: add ssh keys via API registered keys
}
