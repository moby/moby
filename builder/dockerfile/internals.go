package dockerfile

// internals for handling commands. Covers many areas and a lot of
// non-contiguous functionality. Please read the comments.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/containerfs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/go-connections/nat"
	lcUser "github.com/opencontainers/runc/libcontainer/user"
	"github.com/pkg/errors"
)

// Archiver defines an interface for copying files from one destination to
// another using Tar/Untar.
type Archiver interface {
	TarUntar(src, dst string) error
	UntarPath(src, dst string) error
	CopyWithTar(src, dst string) error
	CopyFileWithTar(src, dst string) error
	IDMappings() *idtools.IDMappings
}

// The builder will use the following interfaces if the container fs implements
// these for optimized copies to and from the container.
type extractor interface {
	ExtractArchive(src io.Reader, dst string, opts *archive.TarOptions) error
}

type archiver interface {
	ArchivePath(src string, opts *archive.TarOptions) (io.ReadCloser, error)
}

// helper functions to get tar/untar func
func untarFunc(i interface{}) containerfs.UntarFunc {
	if ea, ok := i.(extractor); ok {
		return ea.ExtractArchive
	}
	return chrootarchive.Untar
}

func tarFunc(i interface{}) containerfs.TarFunc {
	if ap, ok := i.(archiver); ok {
		return ap.ArchivePath
	}
	return archive.TarWithOptions
}

func (b *Builder) getArchiver(src, dst containerfs.Driver) Archiver {
	t, u := tarFunc(src), untarFunc(dst)
	return &containerfs.Archiver{
		SrcDriver:     src,
		DstDriver:     dst,
		Tar:           t,
		Untar:         u,
		IDMappingsVar: b.idMappings,
	}
}

func (b *Builder) commit(dispatchState *dispatchState, comment string) error {
	if b.disableCommit {
		return nil
	}
	if !dispatchState.hasFromImage() {
		return errors.New("Please provide a source image with `from` prior to commit")
	}

	optionsPlatform := system.ParsePlatform(b.options.Platform)
	runConfigWithCommentCmd := copyRunConfig(dispatchState.runConfig, withCmdComment(comment, optionsPlatform.OS))
	hit, err := b.probeCache(dispatchState, runConfigWithCommentCmd)
	if err != nil || hit {
		return err
	}
	id, err := b.create(runConfigWithCommentCmd)
	if err != nil {
		return err
	}

	return b.commitContainer(dispatchState, id, runConfigWithCommentCmd)
}

func (b *Builder) commitContainer(dispatchState *dispatchState, id string, containerConfig *container.Config) error {
	if b.disableCommit {
		return nil
	}

	commitCfg := &backend.ContainerCommitConfig{
		ContainerCommitConfig: types.ContainerCommitConfig{
			Author: dispatchState.maintainer,
			Pause:  true,
			// TODO: this should be done by Commit()
			Config: copyRunConfig(dispatchState.runConfig),
		},
		ContainerConfig: containerConfig,
	}

	// Commit the container
	imageID, err := b.docker.Commit(id, commitCfg)
	if err != nil {
		return err
	}

	dispatchState.imageID = imageID
	return nil
}

func (b *Builder) exportImage(state *dispatchState, imageMount *imageMount, runConfig *container.Config) error {
	optionsPlatform := system.ParsePlatform(b.options.Platform)
	newLayer, err := imageMount.Layer().Commit(optionsPlatform.OS)
	if err != nil {
		return err
	}

	// add an image mount without an image so the layer is properly unmounted
	// if there is an error before we can add the full mount with image
	b.imageSources.Add(newImageMount(nil, newLayer))

	parentImage, ok := imageMount.Image().(*image.Image)
	if !ok {
		return errors.Errorf("unexpected image type")
	}

	newImage := image.NewChildImage(parentImage, image.ChildConfig{
		Author:          state.maintainer,
		ContainerConfig: runConfig,
		DiffID:          newLayer.DiffID(),
		Config:          copyRunConfig(state.runConfig),
	}, parentImage.OS)

	// TODO: it seems strange to marshal this here instead of just passing in the
	// image struct
	config, err := newImage.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "failed to encode image config")
	}

	exportedImage, err := b.docker.CreateImage(config, state.imageID, parentImage.OS)
	if err != nil {
		return errors.Wrapf(err, "failed to export image")
	}

	state.imageID = exportedImage.ImageID()
	b.imageSources.Add(newImageMount(exportedImage, newLayer))
	return nil
}

func (b *Builder) performCopy(state *dispatchState, inst copyInstruction) error {
	srcHash := getSourceHashFromInfos(inst.infos)

	var chownComment string
	if inst.chownStr != "" {
		chownComment = fmt.Sprintf("--chown=%s", inst.chownStr)
	}
	commentStr := fmt.Sprintf("%s %s%s in %s ", inst.cmdName, chownComment, srcHash, inst.dest)

	// TODO: should this have been using origPaths instead of srcHash in the comment?
	optionsPlatform := system.ParsePlatform(b.options.Platform)
	runConfigWithCommentCmd := copyRunConfig(
		state.runConfig,
		withCmdCommentString(commentStr, optionsPlatform.OS))
	hit, err := b.probeCache(state, runConfigWithCommentCmd)
	if err != nil || hit {
		return err
	}

	imageMount, err := b.imageSources.Get(state.imageID, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get destination image %q", state.imageID)
	}

	destInfo, err := createDestInfo(state.runConfig.WorkingDir, inst, imageMount, b.options.Platform)
	if err != nil {
		return err
	}

	chownPair := b.idMappings.RootPair()
	// if a chown was requested, perform the steps to get the uid, gid
	// translated (if necessary because of user namespaces), and replace
	// the root pair with the chown pair for copy operations
	if inst.chownStr != "" {
		chownPair, err = parseChownFlag(inst.chownStr, destInfo.root.Path(), b.idMappings)
		if err != nil {
			return errors.Wrapf(err, "unable to convert uid/gid chown string to host mapping")
		}
	}

	for _, info := range inst.infos {
		opts := copyFileOptions{
			decompress: inst.allowLocalDecompression,
			archiver:   b.getArchiver(info.root, destInfo.root),
			chownPair:  chownPair,
		}
		if err := performCopyForInfo(destInfo, info, opts); err != nil {
			return errors.Wrapf(err, "failed to copy files")
		}
	}
	return b.exportImage(state, imageMount, runConfigWithCommentCmd)
}

func parseChownFlag(chown, ctrRootPath string, idMappings *idtools.IDMappings) (idtools.IDPair, error) {
	var userStr, grpStr string
	parts := strings.Split(chown, ":")
	if len(parts) > 2 {
		return idtools.IDPair{}, errors.New("invalid chown string format: " + chown)
	}
	if len(parts) == 1 {
		// if no group specified, use the user spec as group as well
		userStr, grpStr = parts[0], parts[0]
	} else {
		userStr, grpStr = parts[0], parts[1]
	}

	passwdPath, err := symlink.FollowSymlinkInScope(filepath.Join(ctrRootPath, "etc", "passwd"), ctrRootPath)
	if err != nil {
		return idtools.IDPair{}, errors.Wrapf(err, "can't resolve /etc/passwd path in container rootfs")
	}
	groupPath, err := symlink.FollowSymlinkInScope(filepath.Join(ctrRootPath, "etc", "group"), ctrRootPath)
	if err != nil {
		return idtools.IDPair{}, errors.Wrapf(err, "can't resolve /etc/group path in container rootfs")
	}
	uid, err := lookupUser(userStr, passwdPath)
	if err != nil {
		return idtools.IDPair{}, errors.Wrapf(err, "can't find uid for user "+userStr)
	}
	gid, err := lookupGroup(grpStr, groupPath)
	if err != nil {
		return idtools.IDPair{}, errors.Wrapf(err, "can't find gid for group "+grpStr)
	}

	// convert as necessary because of user namespaces
	chownPair, err := idMappings.ToHost(idtools.IDPair{UID: uid, GID: gid})
	if err != nil {
		return idtools.IDPair{}, errors.Wrapf(err, "unable to convert uid/gid to host mapping")
	}
	return chownPair, nil
}

func lookupUser(userStr, filepath string) (int, error) {
	// if the string is actually a uid integer, parse to int and return
	// as we don't need to translate with the help of files
	uid, err := strconv.Atoi(userStr)
	if err == nil {
		return uid, nil
	}
	users, err := lcUser.ParsePasswdFileFilter(filepath, func(u lcUser.User) bool {
		return u.Name == userStr
	})
	if err != nil {
		return 0, err
	}
	if len(users) == 0 {
		return 0, errors.New("no such user: " + userStr)
	}
	return users[0].Uid, nil
}

func lookupGroup(groupStr, filepath string) (int, error) {
	// if the string is actually a gid integer, parse to int and return
	// as we don't need to translate with the help of files
	gid, err := strconv.Atoi(groupStr)
	if err == nil {
		return gid, nil
	}
	groups, err := lcUser.ParseGroupFileFilter(filepath, func(g lcUser.Group) bool {
		return g.Name == groupStr
	})
	if err != nil {
		return 0, err
	}
	if len(groups) == 0 {
		return 0, errors.New("no such group: " + groupStr)
	}
	return groups[0].Gid, nil
}

func createDestInfo(workingDir string, inst copyInstruction, imageMount *imageMount, platform string) (copyInfo, error) {
	// Twiddle the destination when it's a relative path - meaning, make it
	// relative to the WORKINGDIR
	dest, err := normalizeDest(workingDir, inst.dest, platform)
	if err != nil {
		return copyInfo{}, errors.Wrapf(err, "invalid %s", inst.cmdName)
	}

	destMount, err := imageMount.Source()
	if err != nil {
		return copyInfo{}, errors.Wrapf(err, "failed to mount copy source")
	}

	return newCopyInfoFromSource(destMount, dest, ""), nil
}

// normalizeDest normalises the destination of a COPY/ADD command in a
// platform semantically consistent way.
func normalizeDest(workingDir, requested string, platform string) (string, error) {
	dest := fromSlash(requested, platform)
	endsInSlash := strings.HasSuffix(dest, string(separator(platform)))

	if platform != "windows" {
		if !path.IsAbs(requested) {
			dest = path.Join("/", filepath.ToSlash(workingDir), dest)
			// Make sure we preserve any trailing slash
			if endsInSlash {
				dest += "/"
			}
		}
		return dest, nil
	}

	// We are guaranteed that the working directory is already consistent,
	// However, Windows also has, for now, the limitation that ADD/COPY can
	// only be done to the system drive, not any drives that might be present
	// as a result of a bind mount.
	//
	// So... if the path requested is Linux-style absolute (/foo or \\foo),
	// we assume it is the system drive. If it is a Windows-style absolute
	// (DRIVE:\\foo), error if DRIVE is not C. And finally, ensure we
	// strip any configured working directories drive letter so that it
	// can be subsequently legitimately converted to a Windows volume-style
	// pathname.

	// Not a typo - filepath.IsAbs, not system.IsAbs on this next check as
	// we only want to validate where the DriveColon part has been supplied.
	if filepath.IsAbs(dest) {
		if strings.ToUpper(string(dest[0])) != "C" {
			return "", fmt.Errorf("Windows does not support destinations not on the system drive (C:)")
		}
		dest = dest[2:] // Strip the drive letter
	}

	// Cannot handle relative where WorkingDir is not the system drive.
	if len(workingDir) > 0 {
		if ((len(workingDir) > 1) && !system.IsAbs(workingDir[2:])) || (len(workingDir) == 1) {
			return "", fmt.Errorf("Current WorkingDir %s is not platform consistent", workingDir)
		}
		if !system.IsAbs(dest) {
			if string(workingDir[0]) != "C" {
				return "", fmt.Errorf("Windows does not support relative paths when WORKDIR is not the system drive")
			}
			dest = filepath.Join(string(os.PathSeparator), workingDir[2:], dest)
			// Make sure we preserve any trailing slash
			if endsInSlash {
				dest += string(os.PathSeparator)
			}
		}
	}
	return dest, nil
}

// For backwards compat, if there's just one info then use it as the
// cache look-up string, otherwise hash 'em all into one
func getSourceHashFromInfos(infos []copyInfo) string {
	if len(infos) == 1 {
		return infos[0].hash
	}
	var hashs []string
	for _, info := range infos {
		hashs = append(hashs, info.hash)
	}
	return hashStringSlice("multi", hashs)
}

func hashStringSlice(prefix string, slice []string) string {
	hasher := sha256.New()
	hasher.Write([]byte(strings.Join(slice, ",")))
	return prefix + ":" + hex.EncodeToString(hasher.Sum(nil))
}

type runConfigModifier func(*container.Config)

func withCmd(cmd []string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Cmd = cmd
	}
}

// withCmdComment sets Cmd to a nop comment string. See withCmdCommentString for
// why there are two almost identical versions of this.
func withCmdComment(comment string, platform string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Cmd = append(getShell(runConfig, platform), "#(nop) ", comment)
	}
}

// withCmdCommentString exists to maintain compatibility with older versions.
// A few instructions (workdir, copy, add) used a nop comment that is a single arg
// where as all the other instructions used a two arg comment string. This
// function implements the single arg version.
func withCmdCommentString(comment string, platform string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Cmd = append(getShell(runConfig, platform), "#(nop) "+comment)
	}
}

func withEnv(env []string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Env = env
	}
}

// withEntrypointOverride sets an entrypoint on runConfig if the command is
// not empty. The entrypoint is left unmodified if command is empty.
//
// The dockerfile RUN instruction expect to run without an entrypoint
// so the runConfig entrypoint needs to be modified accordingly. ContainerCreate
// will change a []string{""} entrypoint to nil, so we probe the cache with the
// nil entrypoint.
func withEntrypointOverride(cmd []string, entrypoint []string) runConfigModifier {
	return func(runConfig *container.Config) {
		if len(cmd) > 0 {
			runConfig.Entrypoint = entrypoint
		}
	}
}

func copyRunConfig(runConfig *container.Config, modifiers ...runConfigModifier) *container.Config {
	copy := *runConfig
	copy.Cmd = copyStringSlice(runConfig.Cmd)
	copy.Env = copyStringSlice(runConfig.Env)
	copy.Entrypoint = copyStringSlice(runConfig.Entrypoint)
	copy.OnBuild = copyStringSlice(runConfig.OnBuild)
	copy.Shell = copyStringSlice(runConfig.Shell)

	if copy.Volumes != nil {
		copy.Volumes = make(map[string]struct{}, len(runConfig.Volumes))
		for k, v := range runConfig.Volumes {
			copy.Volumes[k] = v
		}
	}

	if copy.ExposedPorts != nil {
		copy.ExposedPorts = make(nat.PortSet, len(runConfig.ExposedPorts))
		for k, v := range runConfig.ExposedPorts {
			copy.ExposedPorts[k] = v
		}
	}

	if copy.Labels != nil {
		copy.Labels = make(map[string]string, len(runConfig.Labels))
		for k, v := range runConfig.Labels {
			copy.Labels[k] = v
		}
	}

	for _, modifier := range modifiers {
		modifier(&copy)
	}
	return &copy
}

func copyStringSlice(orig []string) []string {
	if orig == nil {
		return nil
	}
	return append([]string{}, orig...)
}

// getShell is a helper function which gets the right shell for prefixing the
// shell-form of RUN, ENTRYPOINT and CMD instructions
func getShell(c *container.Config, os string) []string {
	if 0 == len(c.Shell) {
		return append([]string{}, defaultShellForOS(os)[:]...)
	}
	return append([]string{}, c.Shell[:]...)
}

func (b *Builder) probeCache(dispatchState *dispatchState, runConfig *container.Config) (bool, error) {
	cachedID, err := b.imageProber.Probe(dispatchState.imageID, runConfig)
	if cachedID == "" || err != nil {
		return false, err
	}
	fmt.Fprint(b.Stdout, " ---> Using cache\n")

	dispatchState.imageID = cachedID
	return true, nil
}

var defaultLogConfig = container.LogConfig{Type: "none"}

func (b *Builder) probeAndCreate(dispatchState *dispatchState, runConfig *container.Config) (string, error) {
	if hit, err := b.probeCache(dispatchState, runConfig); err != nil || hit {
		return "", err
	}
	// Set a log config to override any default value set on the daemon
	hostConfig := &container.HostConfig{LogConfig: defaultLogConfig}
	optionsPlatform := system.ParsePlatform(b.options.Platform)
	container, err := b.containerManager.Create(runConfig, hostConfig, optionsPlatform.OS)
	return container.ID, err
}

func (b *Builder) create(runConfig *container.Config) (string, error) {
	hostConfig := hostConfigFromOptions(b.options)
	optionsPlatform := system.ParsePlatform(b.options.Platform)
	container, err := b.containerManager.Create(runConfig, hostConfig, optionsPlatform.OS)
	if err != nil {
		return "", err
	}
	// TODO: could this be moved into containerManager.Create() ?
	for _, warning := range container.Warnings {
		fmt.Fprintf(b.Stdout, " ---> [Warning] %s\n", warning)
	}
	fmt.Fprintf(b.Stdout, " ---> Running in %s\n", stringid.TruncateID(container.ID))
	return container.ID, nil
}

func hostConfigFromOptions(options *types.ImageBuildOptions) *container.HostConfig {
	resources := container.Resources{
		CgroupParent: options.CgroupParent,
		CPUShares:    options.CPUShares,
		CPUPeriod:    options.CPUPeriod,
		CPUQuota:     options.CPUQuota,
		CpusetCpus:   options.CPUSetCPUs,
		CpusetMems:   options.CPUSetMems,
		Memory:       options.Memory,
		MemorySwap:   options.MemorySwap,
		Ulimits:      options.Ulimits,
	}

	return &container.HostConfig{
		SecurityOpt: options.SecurityOpt,
		Isolation:   options.Isolation,
		ShmSize:     options.ShmSize,
		Resources:   resources,
		NetworkMode: container.NetworkMode(options.NetworkMode),
		// Set a log config to override any default value set on the daemon
		LogConfig:  defaultLogConfig,
		ExtraHosts: options.ExtraHosts,
	}
}

// fromSlash works like filepath.FromSlash but with a given OS platform field
func fromSlash(path, platform string) string {
	if platform == "windows" {
		return strings.Replace(path, "/", "\\", -1)
	}
	return path
}

// separator returns a OS path separator for the given OS platform
func separator(platform string) byte {
	if platform == "windows" {
		return '\\'
	}
	return '/'
}
