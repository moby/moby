package daemon

import (
	"fmt"

	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/runconfig"
	containertypes "github.com/docker/engine-api/types/container"
	"runtime"
	"strings"
	"time"
)

// ErrImageDoesNotExist is error returned when no image can be found for a reference.
type ErrImageDoesNotExist struct {
	RefOrID string
}

func (e ErrImageDoesNotExist) Error() string {
	return fmt.Sprintf("no such id: %s", e.RefOrID)
}

// GetImageID returns an image ID corresponding to the image referred to by
// refOrID.
func (daemon *Daemon) GetImageID(refOrID string) (image.ID, error) {
	id, ref, err := reference.ParseIDOrReference(refOrID)
	if err != nil {
		return "", err
	}
	if id != "" {
		if _, err := daemon.imageStore.Get(image.ID(id)); err != nil {
			return "", ErrImageDoesNotExist{refOrID}
		}
		return image.ID(id), nil
	}

	if id, err := daemon.referenceStore.Get(ref); err == nil {
		return id, nil
	}
	if tagged, ok := ref.(reference.NamedTagged); ok {
		if id, err := daemon.imageStore.Search(tagged.Tag()); err == nil {
			for _, namedRef := range daemon.referenceStore.References(id) {
				if namedRef.Name() == ref.Name() {
					return id, nil
				}
			}
		}
	}

	// Search based on ID
	if id, err := daemon.imageStore.Search(refOrID); err == nil {
		return id, nil
	}

	return "", ErrImageDoesNotExist{refOrID}
}

// GetImage returns an image corresponding to the image referred to by refOrID.
func (daemon *Daemon) GetImage(refOrID string) (*image.Image, error) {
	imgID, err := daemon.GetImageID(refOrID)
	if err != nil {
		return nil, err
	}
	return daemon.imageStore.Get(imgID)
}

// GetImageOnBuild looks up a Docker image referenced by `name`.
func (daemon *Daemon) GetImageOnBuild(name string) (builder.Image, error) {
	img, err := daemon.GetImage(name)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// GetCachedImage returns the most recent created image that is a child
// of the image with imgID, that had the same config when it was
// created. nil is returned if a child cannot be found. An error is
// returned if the parent image cannot be found.
func (daemon *Daemon) GetCachedImage(imgID image.ID, config *containertypes.Config) (*image.Image, error) {
	// Loop on the children of the given image and check the config
	getMatch := func(siblings []image.ID) (*image.Image, error) {
		var match *image.Image
		for _, id := range siblings {
			img, err := daemon.imageStore.Get(id)
			if err != nil {
				return nil, fmt.Errorf("unable to find image %q", id)
			}

			if runconfig.Compare(&img.ContainerConfig, config) {
				// check for the most up to date match
				if match == nil || match.Created.Before(img.Created) {
					match = img
				}
			}
		}
		return match, nil
	}

	// In this case, this is `FROM scratch`, which isn't an actual image.
	if imgID == "" {
		images := daemon.imageStore.Map()
		var siblings []image.ID
		for id, img := range images {
			if img.Parent == imgID {
				siblings = append(siblings, id)
			}
		}
		return getMatch(siblings)
	}

	// find match from child images
	siblings := daemon.imageStore.Children(imgID)
	return getMatch(siblings)
}

type daemonImageCacheForBuild struct {
	// cacheFromImages here is a map of (provided) names to the actual images it represents
	cacheFromImages map[string]*image.Image
	// cacheFromImageHistories also provides a map back to a historyWithSourceT struct
	cacheFromImageHistories map[string][]historyWithSourceT
	// daemon stores a reference to the daemon that backs this cache
	daemon *Daemon
}

// MakeImageCacheForBuild creates a stateful image cache that can be used to create images
// using already-existing layers from the cacheFrom images. It needs to be re-created for every
// build because it performs stateful actions such as probing the registry for the specified images.
// At least, it will - at the moment it does none of those things, but we should get the interface
// corect first.
func (daemon *Daemon) MakeImageCacheForBuild(cacheFrom []string) builder.ImageCacheForBuild {
	cache := &daemonImageCacheForBuild{
		daemon:                  daemon,
		cacheFromImages:         make(map[string]*image.Image),
		cacheFromImageHistories: make(map[string][]historyWithSourceT),
	}

	// for each cacheFrom image, set up the channels & coroutine for scrolling forward through
	// its history and comparing it to what's being built
	for _, cacheFromImageName := range cacheFrom {
		cacheFromImage, err := daemon.GetImage(cacheFromImageName)
		if err != nil {
			logrus.Warnf("Could not look up %s for cache resolution, skipping: %s", cacheFromImageName, err)
			continue
		}

		cache.cacheFromImages[cacheFromImageName] = cacheFromImage
		cache.cacheFromImageHistories[cacheFromImageName] = makeHistoryWithSource(cacheFromImage)
	}

	return cache
}

// In the history array, we have pairs of (command, resultingLayerID). What we actually want to be able
// to compare is pairs of (sourceLayerID, command), and if we have a match, consult resultingLayerID.
// We also don't directly have source/resultingLayerID, but rather a boolean "did create new layer" flag.
// Define a struct to store this mapping for convenience.
type historyWithSourceT struct {
	// sourceLayerID is the layer on which the command was run. Empty digest if this is the first command or
	// if nothing has actually been added to the rootfs yet.
	sourceLayerID layer.DiffID
	// cmd is the command which got run on sourceLayerID
	cmd string
	// resulingLayerID is the result of running cmd on sourceLayerID (might be the same as sourceLayerID)
	resultingLayerID layer.DiffID
	// createdAt is the time the history entry was created
	createdAt time.Time
	// history is the actual, underlying history entry
	history image.History
}

func makeHistoryWithSource(image *image.Image) []historyWithSourceT {
	// Let's make those structs now
	historyWithSource := make([]historyWithSourceT, len(image.History))
	layerIndex := -1
	for i, h := range image.History {

		// previous is layerIndex from previous iteration
		if layerIndex == -1 {
			historyWithSource[i].sourceLayerID = digest.DigestSha256EmptyTar
		} else {
			historyWithSource[i].sourceLayerID = image.RootFS.DiffIDs[layerIndex]
		}

		// now increment, if needed, and look at the result layer ID
		if !h.EmptyLayer {
			layerIndex = layerIndex + 1
		}
		if layerIndex == -1 {
			historyWithSource[i].resultingLayerID = digest.DigestSha256EmptyTar
		} else {
			historyWithSource[i].resultingLayerID = image.RootFS.DiffIDs[layerIndex]
		}

		// Copy the other history entries over I'm interested in
		historyWithSource[i].cmd = h.CreatedBy
		historyWithSource[i].createdAt = h.Created
		historyWithSource[i].history = h
	}

	return historyWithSource
}

// GetCachedImageOnBuild returns a reference to a cached image whose parent equals `parent`
// and runconfig equals `cfg`. A cache miss is expected to return an empty ID and a nil error.
func (cache *daemonImageCacheForBuild) GetCachedImageOnBuild(imgID string, cfg *containertypes.Config) (string, error) {
	cachedImage, err := cache.daemon.GetCachedImage(image.ID(imgID), cfg)
	if err != nil {
		return "", err
	}
	if cachedImage != nil {
		// We found a cache hit using the old parent image method
		return cachedImage.ID().String(), nil
	}

	// We didn't find a cache hit using that method. Explore cacheFrom images for matching history
	var parentImage *image.Image
	var parentImageHistory []historyWithSourceT
	// i.e. not FROM SCRATCH
	if imgID != "" {
		parentImage, err = cache.daemon.imageStore.Get(image.ID(imgID))
		if err != nil {
			return "", err
		}
		parentImageHistory = makeHistoryWithSource(parentImage)
	}

	// For each thing we are caching from, see if it matches parentImageHistory
	type matchStruct struct {
		cacheFrom   string
		nextLayerID layer.DiffID
		createdAt   time.Time
		history     image.History
	}
	matches := make([]matchStruct, 0, len(cache.cacheFromImages))
	for cacheFromName, cacheFromImageHistory := range cache.cacheFromImageHistories {

		if historiesMatch(cacheFromImageHistory, parentImageHistory) &&
			// Not only do the histories need to match, but cacheFromImageHistory needs to be long enough
			// that we can peek at the next piece of "history" (which is in the future for parentImage -
			// that's why we can cache from it!)
			(len(cacheFromImageHistory) > len(parentImageHistory)) &&
			// And the command needs to match too.
			(strings.Join(cfg.Cmd, " ") == cacheFromImageHistory[len(parentImageHistory)].cmd) {

			match := matchStruct{
				cacheFrom:   cacheFromName,
				nextLayerID: cacheFromImageHistory[len(parentImageHistory)].resultingLayerID,
				createdAt:   cacheFromImageHistory[len(parentImageHistory)].createdAt,
				history:     cacheFromImageHistory[len(parentImageHistory)].history,
			}
			matches = append(matches, match)
		}
	}

	// Pluck out the newest layer from the potential matches
	// Pointer so it can be nil
	var newestMatch *matchStruct
	for _, match := range matches {
		if newestMatch == nil || match.createdAt.After(newestMatch.createdAt) {
			newestMatch = &match
		}
	}

	// If we have a match, build an image from it to represent the next state of the build
	if newestMatch != nil {
		// TODO: I just copied this from daemon/commit.go. I'm not sure how/if it should be shared.
		// I guess the history should just be the real History entry for the next layer?
		rootFS := image.NewRootFS()
		var oldHistory []image.History
		osVersion := ""
		var osFeatures []string
		var containerConfig containertypes.Config

		if parentImage != nil {
			rootFS = parentImage.RootFS
			oldHistory = parentImage.History
			osVersion = parentImage.OSVersion
			osFeatures = parentImage.OSFeatures
			containerConfig = *parentImage.V1Image.Config
		}

		if !newestMatch.history.EmptyLayer {
			rootFS.Append(newestMatch.nextLayerID)
		}

		newHistory := append(oldHistory, newestMatch.history)

		config, err := json.Marshal(&image.Image{
			V1Image: image.V1Image{
				DockerVersion: dockerversion.Version,
				Config:        cfg,
				Architecture:  runtime.GOARCH,
				OS:            runtime.GOOS,
				// TODO: We didn't really make a container, so I'm just going to leave this emtpy.
				// Container:       container.ID,
				// TODO: I think this is supposed to mean, "the contianerConfig we started with", so this makes sense?
				ContainerConfig: containerConfig,
				// TODO: What should go in here?
				Author:  "",
				Created: newestMatch.history.Created,
			},
			RootFS:     rootFS,
			History:    newHistory,
			OSFeatures: osFeatures,
			OSVersion:  osVersion,
		})
		if err != nil {
			return "", err
		}

		newImageID, err := cache.daemon.imageStore.Create(config)
		if err != nil {
			return "", err
		}

		if parentImage != nil {
			if err := cache.daemon.imageStore.SetParent(newImageID, parentImage.ID()); err != nil {
				return "", err
			}
		}

		// et viola.
		return newImageID.String(), nil
	}

	// Otherwise, cache miss.
	return "", nil
}

func historiesMatch(h1 []historyWithSourceT, h2 []historyWithSourceT) bool {
	if len(h1) <= len(h2) {
		// This won't really work - we have more steps than the cache from image has, so
		// there is no possibility of a match.
		return false
	}
	// Otherwise, let's check that all the commands & layer IDs match
	for i, h2Entry := range h2 {
		h1Entry := h1[i]

		if h1Entry.sourceLayerID != h2Entry.sourceLayerID || h1Entry.cmd != h2Entry.cmd {
			return false
		}
	}
	return true
}
