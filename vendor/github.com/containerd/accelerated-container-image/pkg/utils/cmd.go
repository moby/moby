/*
   Copyright The Accelerated Container Image Authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	sn "github.com/containerd/accelerated-container-image/pkg/types"
	"github.com/containerd/log"
	"github.com/pkg/errors"
)

const (
	obdBinCreate        = "/opt/overlaybd/bin/overlaybd-create"
	obdBinCommit        = "/opt/overlaybd/bin/overlaybd-commit"
	obdBinApply         = "/opt/overlaybd/bin/overlaybd-apply"
	obdBinTurboOCIApply = "/opt/overlaybd/bin/turboOCI-apply"

	dataFile       = "writable_data"
	idxFile        = "writable_index"
	sealedFile     = "overlaybd.sealed"
	commitTempFile = "overlaybd.commit.temp"
	commitFile     = "overlaybd.commit"
)

type ConvertOption struct {
	// src options
	// (TODO) LayerPath   string // path of layer.tgz or layer.tar
	TarMetaPath string // path of layer.tar.meta

	Config  sn.OverlayBDBSConfig
	Workdir string

	// output options
	Ext4FSMetaPath string
	// (TODO) GzipIndexPath string
}

var defaultServiceTemplate = `
{
	"registryFsVersion": "v2",
	"logPath": "",
	"logLevel": 1,
	"cacheConfig": {
		"cacheType": "file",
		"cacheDir": "%s",
		"cacheSizeGB": 4
	},
	"gzipCacheConfig": {
		"enable": false
	},
	"credentialConfig": {
		"mode": "file",
		"path": ""
	},
	"ioEngine": 0,
	"download": {
		"enable": false
	},
	"enableAudit": false
}
`

func Create(ctx context.Context, dir string, opts ...string) error {
	dataPath := path.Join(dir, dataFile)
	indexPath := path.Join(dir, idxFile)
	os.RemoveAll(dataPath)
	os.RemoveAll(indexPath)
	args := append([]string{dataPath, indexPath}, opts...)
	log.G(ctx).Debugf("%s %s", obdBinCreate, strings.Join(args, " "))
	out, err := exec.CommandContext(ctx, obdBinCreate, args...).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to overlaybd-create: %s", out)
	}
	return nil
}

func Seal(ctx context.Context, dir, toDir string, opts ...string) error {
	args := append([]string{
		"--seal",
		path.Join(dir, dataFile),
		path.Join(dir, idxFile),
	}, opts...)
	log.G(ctx).Debugf("%s %s", obdBinCommit, strings.Join(args, " "))
	out, err := exec.CommandContext(ctx, obdBinCommit, args...).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to seal writable overlaybd: %s", out)
	}
	if err := os.Rename(path.Join(dir, dataFile), path.Join(toDir, sealedFile)); err != nil {
		return errors.Wrapf(err, "failed to rename sealed overlaybd file")
	}
	os.RemoveAll(path.Join(dir, idxFile))
	return nil
}

func Commit(ctx context.Context, dir, toDir string, sealed bool, opts ...string) error {
	var args []string
	if sealed {
		args = append([]string{
			"--commit_sealed",
			path.Join(dir, sealedFile),
			path.Join(toDir, commitTempFile),
		}, opts...)
	} else {
		args = append([]string{
			path.Join(dir, dataFile),
			path.Join(dir, idxFile),
			path.Join(toDir, commitFile),
		}, opts...)
	}
	log.G(ctx).Debugf("%s %s", obdBinCommit, strings.Join(args, " "))
	out, err := exec.CommandContext(ctx, obdBinCommit, args...).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to overlaybd-commit: %s", out)
	}
	if sealed {
		return os.Rename(path.Join(toDir, commitTempFile), path.Join(toDir, commitFile))
	}
	return nil
}

func ApplyOverlaybd(ctx context.Context, dir string, opts ...string) error {

	args := append([]string{
		path.Join(dir, "layer.tar"),
		path.Join(dir, "config.json")}, opts...)
	log.G(ctx).Debugf("%s %s", obdBinApply, strings.Join(args, " "))
	out, err := exec.CommandContext(ctx, obdBinApply, args...).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to overlaybd-apply[native]: %s", out)
	}
	return nil
}

func ApplyTurboOCI(ctx context.Context, dir, gzipMetaFile string, opts ...string) error {

	args := append([]string{
		path.Join(dir, "layer.tar"),
		path.Join(dir, "config.json"),
		"--gz_index_path", path.Join(dir, gzipMetaFile)}, opts...)
	log.G(ctx).Debugf("%s %s", obdBinApply, strings.Join(args, " "))
	out, err := exec.CommandContext(ctx, obdBinTurboOCIApply, args...).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to overlaybd-apply[turboOCI]: %s", out)
	}
	return nil
}

func GenerateTarMeta(ctx context.Context, srcTarFile string, dstTarMeta string) error {

	if _, err := os.Stat(srcTarFile); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("error stating tar file: %w", err)
	}
	log.G(ctx).Infof("generate layer meta for %s", srcTarFile)
	if err := exec.Command(obdBinTurboOCIApply, srcTarFile, dstTarMeta, "--export").Run(); err != nil {
		return fmt.Errorf("failed to convert tar file to overlaybd device: %w", err)
	}
	return nil
}

// ConvertLayer produce a turbooci layer, target is path of ext4.fs.meta
func ConvertLayer(ctx context.Context, opt *ConvertOption, fs_type string) error {
	if opt.Workdir == "" {
		opt.Workdir = "tmp_conv"
	}

	if err := os.MkdirAll(opt.Workdir, 0755); err != nil {
		return fmt.Errorf("failed to create work dir: %w", err)
	}

	pathWritableData := filepath.Join(opt.Workdir, "writable_data")
	pathWritableIndex := filepath.Join(opt.Workdir, "writable_index")
	pathFakeTarget := filepath.Join(opt.Workdir, "fake_target")
	pathService := filepath.Join(opt.Workdir, "service.json")
	pathConfig := filepath.Join(opt.Workdir, "config.v1.json")

	// overlaybd-create
	args := []string{pathWritableData, pathWritableIndex, "256", "-s", "--turboOCI"}
	if fs_type != "erofs" && len(opt.Config.Lowers) == 0 {
		args = append(args, "--mkfs")
	}
	if out, err := exec.CommandContext(ctx, obdBinCreate, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to overlaybd-create: %w, output: %s", err, out)
	}
	file, err := os.Create(pathFakeTarget)
	if err != nil {
		return fmt.Errorf("failed to create fake target: %w", err)
	}
	file.Close()
	opt.Config.Upper = sn.OverlayBDBSConfigUpper{
		Data:   pathWritableData,
		Index:  pathWritableIndex,
		Target: pathFakeTarget,
	}

	// turboOCI-apply
	if err := os.WriteFile(pathService, []byte(fmt.Sprintf(defaultServiceTemplate,
		filepath.Join(opt.Workdir, "cache"))), 0644,
	); err != nil {
		return fmt.Errorf("failed to write service.json: %w", err)
	}
	configBytes, err := json.Marshal(opt.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal overlaybd config: %w", err)
	}
	if err := os.WriteFile(pathConfig, configBytes, 0644); err != nil {
		return fmt.Errorf("failed to write overlaybd config: %w", err)
	}
	args = []string{
		opt.TarMetaPath, pathConfig,
		"--service_config_path", pathService,
		"--fstype", fs_type,
	}
	if fs_type != "erofs" {
		args = append(args, "--import")
	}

	log.G(ctx).Debugf("%s %s", obdBinTurboOCIApply, strings.Join(args, " "))
	if out, err := exec.CommandContext(ctx, obdBinTurboOCIApply,
		args...,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to turboOCI-apply: %w, output: %s", err, out)
	}

	// overlaybd-commit
	if out, err := exec.CommandContext(ctx, obdBinCommit,
		pathWritableData,
		pathWritableIndex,
		opt.Ext4FSMetaPath,
		"-z", "--turboOCI",
	).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to overlaybd-commit: %w, output: %s", err, out)
	}
	return nil
}
