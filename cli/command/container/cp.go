package container

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/system"
	"github.com/spf13/cobra"
)

type copyOptions struct {
	source      string
	destination string
	followLink  bool
}

type copyDirection int

const (
	fromContainer copyDirection = (1 << iota)
	toContainer
	acrossContainers = fromContainer | toContainer
)

type cpConfig struct {
	followLink bool
}

// NewCopyCommand creates a new `docker cp` command
func NewCopyCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts copyOptions

	cmd := &cobra.Command{
		Use: `cp [OPTIONS] CONTAINER:SRC_PATH DEST_PATH|-
	docker cp [OPTIONS] SRC_PATH|- CONTAINER:DEST_PATH`,
		Short: "Copy files/folders between a container and the local filesystem",
		Long: strings.Join([]string{
			"Copy files/folders between a container and the local filesystem\n",
			"\nUse '-' as the source to read a tar archive from stdin\n",
			"and extract it to a directory destination in a container.\n",
			"Use '-' as the destination to stream a tar archive of a\n",
			"container source to stdout.",
		}, ""),
		Args: cli.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "" {
				return fmt.Errorf("source can not be empty")
			}
			if args[1] == "" {
				return fmt.Errorf("destination can not be empty")
			}
			opts.source = args[0]
			opts.destination = args[1]
			return runCopy(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.followLink, "follow-link", "L", false, "Always follow symbol link in SRC_PATH")

	return cmd
}

func runCopy(dockerCli *command.DockerCli, opts copyOptions) error {
	srcContainer, srcPath := splitCpArg(opts.source)
	dstContainer, dstPath := splitCpArg(opts.destination)

	var direction copyDirection
	if srcContainer != "" {
		direction |= fromContainer
	}
	if dstContainer != "" {
		direction |= toContainer
	}

	cpParam := &cpConfig{
		followLink: opts.followLink,
	}

	ctx := context.Background()

	switch direction {
	case fromContainer:
		return copyFromContainer(ctx, dockerCli, srcContainer, srcPath, dstPath, cpParam)
	case toContainer:
		return copyToContainer(ctx, dockerCli, srcPath, dstContainer, dstPath, cpParam)
	case acrossContainers:
		// Copying between containers isn't supported.
		return fmt.Errorf("copying between containers is not supported")
	default:
		// User didn't specify any container.
		return fmt.Errorf("must specify at least one container source")
	}
}

func statContainerPath(ctx context.Context, dockerCli *command.DockerCli, containerName, path string) (types.ContainerPathStat, error) {
	return dockerCli.Client().ContainerStatPath(ctx, containerName, path)
}

func resolveLocalPath(localPath string) (absPath string, err error) {
	if absPath, err = filepath.Abs(localPath); err != nil {
		return
	}

	return archive.PreserveTrailingDotOrSeparator(absPath, localPath), nil
}

func copyFromContainer(ctx context.Context, dockerCli *command.DockerCli, srcContainer, srcPath, dstPath string, cpParam *cpConfig) (err error) {
	if dstPath != "-" {
		// Get an absolute destination path.
		dstPath, err = resolveLocalPath(dstPath)
		if err != nil {
			return err
		}
	}

	// if client requests to follow symbol link, then must decide target file to be copied
	var rebaseName string
	if cpParam.followLink {
		srcStat, err := statContainerPath(ctx, dockerCli, srcContainer, srcPath)

		// If the destination is a symbolic link, we should follow it.
		if err == nil && srcStat.Mode&os.ModeSymlink != 0 {
			linkTarget := srcStat.LinkTarget
			if !system.IsAbs(linkTarget) {
				// Join with the parent directory.
				srcParent, _ := archive.SplitPathDirEntry(srcPath)
				linkTarget = filepath.Join(srcParent, linkTarget)
			}

			linkTarget, rebaseName = archive.GetRebaseName(srcPath, linkTarget)
			srcPath = linkTarget
		}

	}

	content, stat, err := dockerCli.Client().CopyFromContainer(ctx, srcContainer, srcPath)
	if err != nil {
		return err
	}
	defer content.Close()

	if dstPath == "-" {
		// Send the response to STDOUT.
		_, err = io.Copy(os.Stdout, content)

		return err
	}

	// Prepare source copy info.
	srcInfo := archive.CopyInfo{
		Path:       srcPath,
		Exists:     true,
		IsDir:      stat.Mode.IsDir(),
		RebaseName: rebaseName,
	}

	preArchive := content
	if len(srcInfo.RebaseName) != 0 {
		_, srcBase := archive.SplitPathDirEntry(srcInfo.Path)
		preArchive = archive.RebaseArchiveEntries(content, srcBase, srcInfo.RebaseName)
	}
	// See comments in the implementation of `archive.CopyTo` for exactly what
	// goes into deciding how and whether the source archive needs to be
	// altered for the correct copy behavior.
	return archive.CopyTo(preArchive, srcInfo, dstPath)
}

func copyToContainer(ctx context.Context, dockerCli *command.DockerCli, srcPath, dstContainer, dstPath string, cpParam *cpConfig) (err error) {
	if srcPath != "-" {
		// Get an absolute source path.
		srcPath, err = resolveLocalPath(srcPath)
		if err != nil {
			return err
		}
	}

	// In order to get the copy behavior right, we need to know information
	// about both the source and destination. The API is a simple tar
	// archive/extract API but we can use the stat info header about the
	// destination to be more informed about exactly what the destination is.

	// Prepare destination copy info by stat-ing the container path.
	dstInfo := archive.CopyInfo{Path: dstPath}
	dstStat, err := statContainerPath(ctx, dockerCli, dstContainer, dstPath)

	// If the destination is a symbolic link, we should evaluate it.
	if err == nil && dstStat.Mode&os.ModeSymlink != 0 {
		linkTarget := dstStat.LinkTarget
		if !system.IsAbs(linkTarget) {
			// Join with the parent directory.
			dstParent, _ := archive.SplitPathDirEntry(dstPath)
			linkTarget = filepath.Join(dstParent, linkTarget)
		}

		dstInfo.Path = linkTarget
		dstStat, err = statContainerPath(ctx, dockerCli, dstContainer, linkTarget)
	}

	// Ignore any error and assume that the parent directory of the destination
	// path exists, in which case the copy may still succeed. If there is any
	// type of conflict (e.g., non-directory overwriting an existing directory
	// or vice versa) the extraction will fail. If the destination simply did
	// not exist, but the parent directory does, the extraction will still
	// succeed.
	if err == nil {
		dstInfo.Exists, dstInfo.IsDir = true, dstStat.Mode.IsDir()
	}

	var (
		content         io.Reader
		resolvedDstPath string
	)

	if srcPath == "-" {
		// Use STDIN.
		content = os.Stdin
		resolvedDstPath = dstInfo.Path
		if !dstInfo.IsDir {
			return fmt.Errorf("destination %q must be a directory", fmt.Sprintf("%s:%s", dstContainer, dstPath))
		}
	} else {
		// Prepare source copy info.
		srcInfo, err := archive.CopyInfoSourcePath(srcPath, cpParam.followLink)
		if err != nil {
			return err
		}

		srcArchive, err := archive.TarResource(srcInfo)
		if err != nil {
			return err
		}
		defer srcArchive.Close()

		// With the stat info about the local source as well as the
		// destination, we have enough information to know whether we need to
		// alter the archive that we upload so that when the server extracts
		// it to the specified directory in the container we get the desired
		// copy behavior.

		// See comments in the implementation of `archive.PrepareArchiveCopy`
		// for exactly what goes into deciding how and whether the source
		// archive needs to be altered for the correct copy behavior when it is
		// extracted. This function also infers from the source and destination
		// info which directory to extract to, which may be the parent of the
		// destination that the user specified.
		dstDir, preparedArchive, err := archive.PrepareArchiveCopy(srcArchive, srcInfo, dstInfo)
		if err != nil {
			return err
		}
		defer preparedArchive.Close()

		resolvedDstPath = dstDir
		content = preparedArchive
	}

	options := types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
	}

	return dockerCli.Client().CopyToContainer(ctx, dstContainer, resolvedDstPath, content, options)
}

// We use `:` as a delimiter between CONTAINER and PATH, but `:` could also be
// in a valid LOCALPATH, like `file:name.txt`. We can resolve this ambiguity by
// requiring a LOCALPATH with a `:` to be made explicit with a relative or
// absolute path:
// 	`/path/to/file:name.txt` or `./file:name.txt`
//
// This is apparently how `scp` handles this as well:
// 	http://www.cyberciti.biz/faq/rsync-scp-file-name-with-colon-punctuation-in-it/
//
// We can't simply check for a filepath separator because container names may
// have a separator, e.g., "host0/cname1" if container is in a Docker cluster,
// so we have to check for a `/` or `.` prefix. Also, in the case of a Windows
// client, a `:` could be part of an absolute Windows path, in which case it
// is immediately proceeded by a backslash.
func splitCpArg(arg string) (container, path string) {
	if system.IsAbs(arg) {
		// Explicit local absolute path, e.g., `C:\foo` or `/foo`.
		return "", arg
	}

	parts := strings.SplitN(arg, ":", 2)

	if len(parts) == 1 || strings.HasPrefix(parts[0], ".") {
		// Either there's no `:` in the arg
		// OR it's an explicit local relative path like `./file:name.txt`.
		return "", arg
	}

	return parts[0], parts[1]
}
