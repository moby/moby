package main

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/term"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/storage/v1"
)

const pollingInterval = 500 * time.Millisecond
const timeout = 300

// GCPClient contains state required for communication with GCP
type GCPClient struct {
	client      *http.Client
	compute     *compute.Service
	storage     *storage.Service
	projectName string
	fileName    string
	privKey     *rsa.PrivateKey
}

// NewGCPClient creates a new GCP client
func NewGCPClient(keys, projectName string) (*GCPClient, error) {
	log.Debugf("Connecting to GCP")
	ctx := context.Background()
	var client *GCPClient
	if keys != "" {
		log.Debugf("Using Keys %s", keys)
		f, err := os.Open(keys)
		if err != nil {
			return nil, err
		}

		jsonKey, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}

		config, err := google.JWTConfigFromJSON(jsonKey,
			storage.DevstorageReadWriteScope,
			compute.ComputeScope,
		)
		if err != nil {
			return nil, err
		}

		client = &GCPClient{
			client:      config.Client(ctx),
			projectName: projectName,
		}
	} else {
		log.Debugf("Using Application Default crednetials")
		gc, err := google.DefaultClient(
			ctx,
			storage.DevstorageReadWriteScope,
			compute.ComputeScope,
		)
		if err != nil {
			return nil, err
		}
		client = &GCPClient{
			client:      gc,
			projectName: projectName,
		}
	}

	var err error
	client.compute, err = compute.New(client.client)
	if err != nil {
		return nil, err
	}

	client.storage, err = storage.New(client.client)
	if err != nil {
		return nil, err
	}

	log.Debugf("Generating SSH Keypair")
	client.privKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// UploadFile uploads a file to Google Storage
func (g GCPClient) UploadFile(src, dst, bucketName string, public bool) error {
	log.Infof("Uploading file %s to Google Storage as %s", src, dst)
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	objectCall := g.storage.Objects.Insert(bucketName, &storage.Object{Name: dst}).Media(f)

	if public {
		objectCall.PredefinedAcl("publicRead")
	}

	_, err = objectCall.Do()
	if err != nil {
		return err
	}
	log.Infof("Upload Complete!")
	fmt.Println("gs://" + bucketName + "/" + dst)
	return nil
}

// CreateImage creates a GCP image using the a source from Google Storage
func (g GCPClient) CreateImage(name, storageURL, family string, replace bool) error {
	if replace {
		if err := g.DeleteImage(name); err != nil {
			return err
		}
	}

	log.Infof("Creating image: %s", name)
	imgObj := &compute.Image{
		RawDisk: &compute.ImageRawDisk{
			Source: storageURL,
		},
		Name: name,
	}

	if family != "" {
		imgObj.Family = family
	}

	op, err := g.compute.Images.Insert(g.projectName, imgObj).Do()
	if err != nil {
		return err
	}

	if err := g.pollOperationStatus(op.Name); err != nil {
		return err
	}
	log.Infof("Image %s created", name)
	return nil
}

// DeleteImage deletes and image
func (g GCPClient) DeleteImage(name string) error {
	var notFound bool
	op, err := g.compute.Images.Delete(g.projectName, name).Do()
	if err != nil {
		if _, ok := err.(*googleapi.Error); !ok {
			return err
		}
		if err.(*googleapi.Error).Code != 404 {
			return err
		}
		notFound = true
	}
	if !notFound {
		log.Infof("Deleting existing image...")
		if err := g.pollOperationStatus(op.Name); err != nil {
			return err
		}
		log.Infof("Image %s deleted", name)
	}
	return nil
}

// CreateInstance creates and starts an instance on GCP
func (g GCPClient) CreateInstance(name, image, zone, machineType string, replace bool) error {
	if replace {
		if err := g.DeleteInstance(name, zone, true); err != nil {
			return err
		}
	}

	log.Infof("Creating instance %s from image %s", name, image)
	enabled := new(string)
	*enabled = "1"

	k, err := ssh.NewPublicKey(g.privKey.Public())
	if err != nil {
		return err
	}
	sshKey := new(string)
	*sshKey = fmt.Sprintf("moby:%s moby", string(ssh.MarshalAuthorizedKey(k)))

	instanceObj := &compute.Instance{
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", zone, machineType),
		Name:        name,
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					SourceImage: fmt.Sprintf("global/images/%s", image),
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network: "global/networks/default",
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					Key:   "serial-port-enable",
					Value: enabled,
				},
				{
					Key:   "ssh-keys",
					Value: sshKey,
				},
			},
		},
	}

	// Don't wait for operation to complete!
	// A headstart is needed as by the time we've polled for this event to be
	// completed, the instance may have already terminated
	_, err = g.compute.Instances.Insert(g.projectName, zone, instanceObj).Do()
	if err != nil {
		return err
	}
	log.Infof("Instance created")
	return nil
}

// DeleteInstance removes an instance
func (g GCPClient) DeleteInstance(instance, zone string, wait bool) error {
	var notFound bool
	op, err := g.compute.Instances.Delete(g.projectName, zone, instance).Do()
	if err != nil {
		if _, ok := err.(*googleapi.Error); !ok {
			return err
		}
		if err.(*googleapi.Error).Code != 404 {
			return err
		}
		notFound = true
	}
	if !notFound && wait {
		log.Infof("Deleting existing instance...")
		if err := g.pollZoneOperationStatus(op.Name, zone); err != nil {
			return err
		}
		log.Infof("Instance %s deleted", instance)
	}
	return nil
}

// GetInstanceSerialOutput streams the serial output of an instance
func (g GCPClient) GetInstanceSerialOutput(instance, zone string) error {
	log.Infof("Getting serial port output for instance %s", instance)
	var next int64
	for {
		res, err := g.compute.Instances.GetSerialPortOutput(g.projectName, zone, instance).Start(next).Do()
		if err != nil {
			if err.(*googleapi.Error).Code == 400 {
				// Instance may not be ready yet...
				time.Sleep(pollingInterval)
				continue
			}
			if err.(*googleapi.Error).Code == 503 {
				// Timeout received when the instance has terminated
				break
			}
			return err
		}
		fmt.Printf(res.Contents)
		next = res.Next
		// When the instance has been stopped, Start and Next will both be 0
		if res.Start > 0 && next == 0 {
			break
		}
	}
	return nil
}

// ConnectToInstanceSerialPort uses SSH to connect to the serial port of the instance
func (g GCPClient) ConnectToInstanceSerialPort(instance, zone string) error {
	log.Infof("Connecting to serial port of instance %s", instance)
	gPubKeyURL := "https://cloud-certs.storage.googleapis.com/google-cloud-serialport-host-key.pub"
	resp, err := http.Get(gPubKeyURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	gPubKey, _, _, _, err := ssh.ParseAuthorizedKey(body)
	if err != nil {
		return err
	}

	signer, err := ssh.NewSignerFromKey(g.privKey)
	if err != nil {
		return err
	}
	config := &ssh.ClientConfig{
		User: fmt.Sprintf("%s.%s.%s.moby", g.projectName, zone, instance),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.FixedHostKey(gPubKey),
		Timeout:         5 * time.Second,
	}

	var conn *ssh.Client
	// Retry connection as VM may not be ready yet
	for i := 0; i < timeout; i++ {
		conn, err = ssh.Dial("tcp", "ssh-serialport.googleapis.com:9600", config)
		if err != nil {
			time.Sleep(pollingInterval)
			continue
		}
		break
	}
	if conn == nil {
		return fmt.Errorf(err.Error())
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stdin for session: %v", err)
	}
	go io.Copy(stdin, os.Stdin)

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stdout for session: %v", err)
	}
	go io.Copy(os.Stdout, stdout)

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stderr for session: %v", err)
	}
	go io.Copy(os.Stderr, stderr)
	/*
		c := make(chan os.Signal, 1)
		exit := make(chan bool, 1)
		signal.Notify(c)
		go func(exit <-chan bool, c <-chan os.Signal) {
			select {
			case <-exit:
				return
			case s := <-c:
				switch s {
				// CTRL+C
				case os.Interrupt:
					session.Signal(ssh.SIGINT)
				// CTRL+\
				case os.Kill:
					session.Signal(ssh.SIGQUIT)
				default:
					log.Debugf("Received signal %s but not forwarding to ssh", s)
				}
			}
		}(exit, c)
	*/
	var termWidth, termHeight int
	fd := os.Stdin.Fd()

	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return err
		}

		defer term.RestoreTerminal(fd, oldState)

		winsize, err := term.GetWinsize(fd)
		if err != nil {
			termWidth = 80
			termHeight = 24
		} else {
			termWidth = int(winsize.Width)
			termHeight = int(winsize.Height)
		}
	}

	if err = session.RequestPty("xterm", termHeight, termWidth, ssh.TerminalModes{
		ssh.ECHO: 1,
	}); err != nil {
		return err
	}

	if err = session.Shell(); err != nil {
		return err
	}

	err = session.Wait()
	//exit <- true
	if err != nil {
		return err
	}
	return nil
}

func (g *GCPClient) pollOperationStatus(operationName string) error {
	for i := 0; i < timeout; i++ {
		operation, err := g.compute.GlobalOperations.Get(g.projectName, operationName).Do()
		if err != nil {
			return fmt.Errorf("error fetching operation status: %v", err)
		}
		if operation.Error != nil {
			return fmt.Errorf("error running operation: %v", operation.Error)
		}
		if operation.Status == "DONE" {
			return nil
		}
		time.Sleep(pollingInterval)
	}
	return fmt.Errorf("timeout waiting for operation to finish")

}
func (g *GCPClient) pollZoneOperationStatus(operationName, zone string) error {
	for i := 0; i < timeout; i++ {
		operation, err := g.compute.ZoneOperations.Get(g.projectName, zone, operationName).Do()
		if err != nil {
			return fmt.Errorf("error fetching operation status: %v", err)
		}
		if operation.Error != nil {
			return fmt.Errorf("error running operation: %v", operation.Error)
		}
		if operation.Status == "DONE" {
			return nil
		}
		time.Sleep(pollingInterval)
	}
	return fmt.Errorf("timeout waiting for operation to finish")
}
