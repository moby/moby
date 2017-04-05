package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/storage/v1"
)

// GCPClient contains state required for communication with GCP
type GCPClient struct {
	client      *http.Client
	compute     *compute.Service
	storage     *storage.Service
	projectName string
	fileName    string
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

	return client, nil
}

// UploadFile uploads a file to Google Storage
func (g GCPClient) UploadFile(filename, bucketName string, public bool) error {
	log.Infof("Uploading file %s to Google Storage", filename)
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	objectCall := g.storage.Objects.Insert(bucketName, &storage.Object{Name: filename}).Media(f)

	if public {
		objectCall.PredefinedAcl("publicRead")
	}

	_, err = objectCall.Do()
	if err != nil {
		return err
	}
	log.Infof("Upload Complete!")
	fmt.Println("gs://" + bucketName + "/" + filename)
	return nil
}

// CreateImage creates a GCE image using the a source from Google Storage
func (g GCPClient) CreateImage(filename, storageURL, family string, replace bool) error {
	if replace {
		var notFound bool
		op, err := g.compute.Images.Delete(g.projectName, filename).Do()
		if err != nil {
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
			log.Infof("Image %s deleted", filename)
		}
	}

	log.Infof("Creating image: %s", filename)
	imgObj := &compute.Image{
		RawDisk: &compute.ImageRawDisk{
			Source: storageURL,
		},
		Name: filename,
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
	log.Infof("Image %s created", filename)
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
