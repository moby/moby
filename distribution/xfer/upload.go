package xfer

import (
	"errors"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/progress"
	"golang.org/x/net/context"
)

const maxUploadAttempts = 5

// LayerUploadManager provides task management and progress reporting for
// uploads.
type LayerUploadManager struct {
	tm TransferManager
}

// NewLayerUploadManager returns a new LayerUploadManager.
func NewLayerUploadManager(concurrencyLimit int) *LayerUploadManager {
	return &LayerUploadManager{
		tm: NewTransferManager(concurrencyLimit),
	}
}

type uploadTransfer struct {
	Transfer

	diffID layer.DiffID
	digest digest.Digest
	err    error
}

// An UploadDescriptor references a layer that may need to be uploaded.
type UploadDescriptor interface {
	// Key returns the key used to deduplicate uploads.
	Key() string
	// ID returns the ID for display purposes.
	ID() string
	// DiffID should return the DiffID for this layer.
	DiffID() layer.DiffID
	// Upload is called to perform the Upload.
	Upload(ctx context.Context, progressOutput progress.Output) (digest.Digest, error)
}

// Upload is a blocking function which ensures the listed layers are present on
// the remote registry. It uses the string returned by the Key method to
// deduplicate uploads.
func (lum *LayerUploadManager) Upload(ctx context.Context, layers []UploadDescriptor, progressOutput progress.Output) (map[layer.DiffID]digest.Digest, error) {
	var (
		uploads          []*uploadTransfer
		digests          = make(map[layer.DiffID]digest.Digest)
		dedupDescriptors = make(map[string]struct{})
	)

	for _, descriptor := range layers {
		progress.Update(progressOutput, descriptor.ID(), "Preparing")

		key := descriptor.Key()
		if _, present := dedupDescriptors[key]; present {
			continue
		}
		dedupDescriptors[key] = struct{}{}

		xferFunc := lum.makeUploadFunc(descriptor)
		upload, watcher := lum.tm.Transfer(descriptor.Key(), xferFunc, progressOutput)
		defer upload.Release(watcher)
		uploads = append(uploads, upload.(*uploadTransfer))
	}

	for _, upload := range uploads {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-upload.Transfer.Done():
			if upload.err != nil {
				return nil, upload.err
			}
			digests[upload.diffID] = upload.digest
		}
	}

	return digests, nil
}

func (lum *LayerUploadManager) makeUploadFunc(descriptor UploadDescriptor) DoFunc {
	return func(progressChan chan<- progress.Progress, start <-chan struct{}, inactive chan<- struct{}) Transfer {
		u := &uploadTransfer{
			Transfer: NewTransfer(),
			diffID:   descriptor.DiffID(),
		}

		go func() {
			defer func() {
				close(progressChan)
			}()

			progressOutput := progress.ChanOutput(progressChan)

			select {
			case <-start:
			default:
				progress.Update(progressOutput, descriptor.ID(), "Waiting")
				<-start
			}

			retries := 0
			for {
				digest, err := descriptor.Upload(u.Transfer.Context(), progressOutput)
				if err == nil {
					u.digest = digest
					break
				}

				// If an error was returned because the context
				// was cancelled, we shouldn't retry.
				select {
				case <-u.Transfer.Context().Done():
					u.err = err
					return
				default:
				}

				retries++
				if _, isDNR := err.(DoNotRetry); isDNR || retries == maxUploadAttempts {
					logrus.Errorf("Upload failed: %v", err)
					u.err = err
					return
				}

				logrus.Errorf("Upload failed, retrying: %v", err)
				delay := retries * 5
				ticker := time.NewTicker(time.Second)

			selectLoop:
				for {
					progress.Updatef(progressOutput, descriptor.ID(), "Retrying in %d seconds", delay)
					select {
					case <-ticker.C:
						delay--
						if delay == 0 {
							ticker.Stop()
							break selectLoop
						}
					case <-u.Transfer.Context().Done():
						ticker.Stop()
						u.err = errors.New("upload cancelled during retry delay")
						return
					}
				}
			}
		}()

		return u
	}
}
