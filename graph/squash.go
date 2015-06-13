package graph

import (
	"errors"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/graph/tags"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
)

// Squash creates a new image that is a combination of all rootfs layers
// between the image specified by the given name or ID and the image specified
// by the given ancestor name or ID. The new image will have the specified
// ancestor image as its direct parent. If *ancestorName* is empty then all
// layers are are merged. The original image and layers are unaltered.
func (store *TagStore) Squash(imgName, ancestorName, repoTag string) (*image.Image, error) {
	repoName, tag := parsers.ParseRepositoryTag(repoTag)
	if repoName != "" {
		if err := registry.ValidateRepositoryName(repoName); err != nil {
			return nil, err
		}
		if len(tag) > 0 {
			if err := tags.ValidateTagName(tag); err != nil {
				return nil, err
			}
		}
	}

	img, err := store.LookupImage(imgName)
	if err != nil {
		return nil, err
	}

	var ancestor *image.Image
	if ancestorName != "" {
		if ancestor, err = store.LookupImage(ancestorName); err != nil {
			return nil, err
		}
	}

	newImg, err := store.graph.Squash(img, ancestor)
	if err != nil {
		return nil, err
	}

	if repoName != "" {
		if err := store.Tag(repoName, tag, newImg.ID, true); err != nil {
			return nil, fmt.Errorf("unable to tag id %s: %v", newImg.ID, err)
		}
	}

	return newImg, nil
}

// Squash consolidates all layers in between the given image and an ancestor
// image into a single layer. Returns the newly created image. A nil ancestor
// argument indicates that the layer should be squashed all the way to a single
// new base image.
func (graph *Graph) Squash(img, ancestor *image.Image) (squashedImage *image.Image, err error) {
	if ancestor != nil && img.ID == ancestor.ID {
		// No need to squash! They are the same image already!
		return img, nil
	}

	var (
		ancestorID string
		squashDiff archive.Archive
	)

	if ancestor != nil {
		ancestorID = ancestor.ID
	}

	log.Debugf("squashing image ID %q up to %q", img.ID, ancestorID)

	if img.Parent == ancestorID {
		// No need to squash! It's already only a single layer difference!
		return img, nil
	}

	// We need to preserve the command history and comments of all the squashed
	// layers.
	var commands, comments []string

	// Ensure that the ancestorID is an actual ancestor by walking the
	// image history to find it. We'll use this sentinel error to exit the
	// walk early and indicate that the ancestor was found. While walking the
	// history, also add the commands to slice of squashed commands.
	foundAncestor := errors.New("found ancestor")
	err = graph.WalkHistory(img, func(img image.Image) error {
		cmdString := "()"
		if img.ContainerConfig.Cmd != nil {
			cmdString = fmt.Sprintf("(%s)", img.ContainerConfig.Cmd.ToString())
		}

		commands = append(commands, cmdString)
		comments = append(comments, fmt.Sprintf("(%q)", img.Comment))

		if img.Parent == ancestorID {
			return foundAncestor // Exit walk early.
		}
		return nil
	})
	if err == nil {
		return nil, fmt.Errorf("unable to squash: %q is not an ancestor of %q", ancestorID, img.ID)
	}
	if err != foundAncestor {
		return nil, err
	}

	// Reverse the commands and comments so that they read better as the image
	// history.
	for i := 0; i < len(commands)/2; i++ {
		swapIdx := len(commands) - i - 1
		commands[i], commands[swapIdx] = commands[swapIdx], commands[i]
		comments[i], comments[swapIdx] = comments[swapIdx], comments[i]
	}

	// The special sauce! Actually getting an diff from the ancestor Image rootfs.
	if squashDiff, err = graph.driver.DiffAncestor(img.ID, ancestorID); err != nil {
		return nil, err
	}
	defer squashDiff.Close()

	// Create a spoofed container config for preserved command history.
	spoofedConfig := &runconfig.Config{
		Cmd: runconfig.NewCommand(append([]string{"/bin/sh", "-c", "#(nop)", "SQUASH"}, strings.Join(commands, ", "))...),
	}
	comment := fmt.Sprintf("Squashed layer comments: %s", strings.Join(comments, ", "))

	// Create it like any other image is created!
	if squashedImage, err = graph.Create(squashDiff, "", ancestorID, comment, img.Author, spoofedConfig, img.Config); err != nil {
		return nil, err
	}

	return squashedImage, nil
}
