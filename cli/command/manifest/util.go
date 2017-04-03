package manifest

// list of valid os/arch values (see "Optional Environment Variables" section
// of https://golang.org/doc/install/source
// Added linux/s390x as we know System z support already exists

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/homedir"
	//"github.com/opencontainers/go-digest"
	//"github.com/docker/distribution/manifest/manifestlist"
)

type osArch struct {
	os   string
	arch string
}

//Remove any unsupported os/arch combo
var validOSArches = map[osArch]bool{
	osArch{os: "darwin", arch: "386"}:      true,
	osArch{os: "darwin", arch: "amd64"}:    true,
	osArch{os: "darwin", arch: "arm"}:      true,
	osArch{os: "darwin", arch: "arm64"}:    true,
	osArch{os: "dragonfly", arch: "amd64"}: true,
	osArch{os: "freebsd", arch: "386"}:     true,
	osArch{os: "freebsd", arch: "amd64"}:   true,
	osArch{os: "freebsd", arch: "arm"}:     true,
	osArch{os: "linux", arch: "386"}:       true,
	osArch{os: "linux", arch: "amd64"}:     true,
	osArch{os: "linux", arch: "arm"}:       true,
	osArch{os: "linux", arch: "arm64"}:     true,
	osArch{os: "linux", arch: "ppc64"}:     true,
	osArch{os: "linux", arch: "ppc64le"}:   true,
	osArch{os: "linux", arch: "mips64"}:    true,
	osArch{os: "linux", arch: "mips64le"}:  true,
	osArch{os: "linux", arch: "s390x"}:     true,
	osArch{os: "netbsd", arch: "386"}:      true,
	osArch{os: "netbsd", arch: "amd64"}:    true,
	osArch{os: "netbsd", arch: "arm"}:      true,
	osArch{os: "openbsd", arch: "386"}:     true,
	osArch{os: "openbsd", arch: "amd64"}:   true,
	osArch{os: "openbsd", arch: "arm"}:     true,
	osArch{os: "plan9", arch: "386"}:       true,
	osArch{os: "plan9", arch: "amd64"}:     true,
	osArch{os: "solaris", arch: "amd64"}:   true,
	osArch{os: "windows", arch: "386"}:     true,
	osArch{os: "windows", arch: "amd64"}:   true,
}

func isValidOSArch(os string, arch string) bool {
	// check for existence of this combo
	_, ok := validOSArches[osArch{os, arch}]
	return ok
}

func makeFilesafeName(ref string) string {
	// Make sure the ref is a normalized name before calling this func
	fileName := strings.Replace(ref, ":", "-", -1)
	return strings.Replace(fileName, "/", "_", -1)
}

func getListFilenames(transaction string) ([]string, error) {
	baseDir, err := buildBaseFilename()
	if err != nil {
		return nil, err
	}
	transactionDir := filepath.Join(baseDir, makeFilesafeName(transaction))
	if err != nil {
		return nil, err
	}
	fd, err := os.Open(transactionDir)
	if err != nil {
		return nil, err
	}
	fileNames, err := fd.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	fd.Close()
	for i, f := range fileNames {
		fileNames[i] = filepath.Join(transactionDir, f)
	}
	return fileNames, nil
}

func getManifestFd(manifest, transaction string) (*os.File, error) {

	fileName, err := mfToFilename(manifest, transaction)
	if err != nil {
		return nil, err
	}

	return getFdGeneric(fileName)
}

func getFdGeneric(file string) (*os.File, error) {
	_, err := os.Stat(file)
	if err != nil && os.IsNotExist(err) {
		logrus.Debugf("Manifest file %s not found.", file)
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	fd, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return fd, nil
}

func buildBaseFilename() (string, error) {
	// Store the manifests in a user's home to prevent conflict. The HOME dir needs to be set,
	// but can only be forcibly set on Linux at this time.
	// See https://github.com/docker/docker/pull/29478 for more background on why this approach
	// is being used.
	if err := ensureHomeIfIAmStatic(); err != nil {
		return "", err
	}
	userHome := homedir.Get()
	return filepath.Join(userHome, ".docker", "manifests"), nil
}

func mfToFilename(manifest, transaction string) (string, error) {

	baseDir, err := buildBaseFilename()
	if err != nil {
		return "", nil
	}
	return filepath.Join(baseDir, makeFilesafeName(transaction), makeFilesafeName(manifest)), nil
}

func unmarshalIntoManifestInspect(manifest, transaction string) (ImgManifestInspect, error) {

	var newMf ImgManifestInspect
	filename, err := mfToFilename(manifest, transaction)
	if err != nil {
		return ImgManifestInspect{}, err
	}
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return ImgManifestInspect{}, err
	}

	if err := json.Unmarshal(buf, &newMf); err != nil {
		return ImgManifestInspect{}, err
	}

	return newMf, nil
}

func updateMfFile(newMf ImgManifestInspect, mfName, transaction string) error {
	fileName, err := mfToFilename(mfName, transaction)
	if err != nil {
		return err
	}
	if err := os.Remove(fileName); err != nil && !os.IsNotExist(err) {
		return err
	}
	fd, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer fd.Close()
	theBytes, err := json.Marshal(newMf)
	if err != nil {
		return err
	}

	if _, err := fd.Write(theBytes); err != nil {
		return err
	}
	return nil
}
