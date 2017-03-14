package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
)

func uploadGS(filename, project, bucket string, public bool) error {
	if project != "" {
		err := os.Setenv("GOOGLE_CLOUD_PROJECT", project)
		if err != nil {
			return err
		}
	}
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		return errors.New("GOOGLE_CLOUD_PROJECT environment variable must be set or project specified in config")
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	wc := client.Bucket(bucket).Object(filename).NewWriter(ctx)
	_, err = io.Copy(wc, f)
	if err != nil {
		return err
	}
	err = wc.Close()
	if err != nil {
		return err
	}

	// TODO make public if requested

	fmt.Println("gs://" + bucket + "/" + filename)

	return nil
}

func imageGS(filename, project, storage string) error {
	if project != "" {
		err := os.Setenv("GOOGLE_CLOUD_PROJECT", project)
		if err != nil {
			return err
		}
	}
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		return errors.New("GOOGLE_CLOUD_PROJECT environment variable must be set or project specified in config")
	}

	// TODO do not shell out to gcloud tool, use the API

	gcloud, err := exec.LookPath("gcloud")
	if err != nil {
		return errors.New("Please install the gcloud binary")
	}
	args := []string{"compute", "images", "create", "--source-uri", storage, filename}
	cmd := exec.Command(gcloud, args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Image creation failed: %v - %s", err, string(out))
	}

	fmt.Println(filename)

	return nil
}
