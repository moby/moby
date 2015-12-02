package native

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func genTmpfsPremountCmd(tmpDir string, fullDest string, dest string) []configs.Command {
	var premount []configs.Command
	tarPath, err := exec.LookPath("tar")
	if err != nil {
		logrus.Warn("tar command is not available for tmpfs mount: %s", err)
		return premount
	}
	if _, err = exec.LookPath("rm"); err != nil {
		logrus.Warn("rm command is not available for tmpfs mount: %s", err)
		return premount
	}
	tarFile := fmt.Sprintf("%s/%s.tar", tmpDir, strings.Replace(dest, "/", "_", -1))
	if _, err := os.Stat(fullDest); err == nil {
		premount = append(premount, configs.Command{
			Path: tarPath,
			Args: []string{"-cf", tarFile, "-C", fullDest, "."},
		})
	}
	return premount
}

func genTmpfsPostmountCmd(tmpDir string, fullDest string, dest string) []configs.Command {
	var postmount []configs.Command
	tarPath, err := exec.LookPath("tar")
	if err != nil {
		return postmount
	}
	rmPath, err := exec.LookPath("rm")
	if err != nil {
		return postmount
	}
	if _, err := os.Stat(fullDest); os.IsNotExist(err) {
		return postmount
	}
	tarFile := fmt.Sprintf("%s/%s.tar", tmpDir, strings.Replace(dest, "/", "_", -1))
	postmount = append(postmount, configs.Command{
		Path: tarPath,
		Args: []string{"-xf", tarFile, "-C", fullDest, "."},
	})
	return append(postmount, configs.Command{
		Path: rmPath,
		Args: []string{"-f", tarFile},
	})
}
