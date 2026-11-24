/*
   Copyright The containerd Authors.

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

package verity

import (
	"bytes"
	"crypto"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/containerd/go-dmverity/pkg/dm"
	"github.com/containerd/go-dmverity/pkg/keyring"
	"github.com/containerd/go-dmverity/pkg/utils"
)

func validateParams(params *Params, digestSize int) error {
	if params == nil {
		return errors.New("verity: nil params")
	}

	if params.HashType > VerityMaxHashType {
		return fmt.Errorf("verity: unsupported hash type %d", params.HashType)
	}
	if params.HashName == "" {
		return errors.New("verity: hash algorithm required")
	}

	if params.SaltSize > MaxSaltSize {
		return fmt.Errorf("salt size %d exceeds maximum of %d bytes", params.SaltSize, MaxSaltSize)
	}

	if digestSize > VerityMaxDigestSize {
		return fmt.Errorf("digest size %d exceeds maximum of %d bytes", digestSize, VerityMaxDigestSize)
	}

	if !utils.IsBlockSizeValid(params.DataBlockSize) {
		return fmt.Errorf("invalid data block size: %d", params.DataBlockSize)
	}
	if !utils.IsBlockSizeValid(params.HashBlockSize) {
		return fmt.Errorf("invalid hash block size: %d", params.HashBlockSize)
	}

	if utils.Uint64MultOverflow(params.DataBlocks, uint64(params.DataBlockSize)) {
		return fmt.Errorf("data device offset overflow: %d blocks * %d bytes",
			params.DataBlocks, params.DataBlockSize)
	}

	if params.NoSuperblock {
		if params.HashAreaOffset%uint64(params.HashBlockSize) != 0 {
			return fmt.Errorf("hash offset %d must be aligned to hash block size %d", params.HashAreaOffset, params.HashBlockSize)
		}
	} else {
		if params.HashAreaOffset == 0 {
			return errors.New("verity: hash area offset not initialised for superblock mode")
		}
	}

	pageSize := uint32(unix.Getpagesize())
	if params.DataBlockSize > pageSize {
		log.Printf("WARNING: Kernel cannot activate device if data block size (%d) exceeds page size (%d)",
			params.DataBlockSize, pageSize)
	}

	return nil
}

func Verify(params *Params, dataDevice, hashDevice string, rootHash []byte) error {
	if params == nil {
		return errors.New("verity: nil params")
	}

	if !params.NoSuperblock {
		hashFile, err := os.Open(hashDevice)
		if err != nil {
			return fmt.Errorf("cannot open hash device: %w", err)
		}
		defer hashFile.Close()

		sbOffset := uint64(0)
		if dataDevice == hashDevice && params.HashAreaOffset > 0 {
			sbOffset = params.HashAreaOffset
		}

		sb, err := ReadSuperblock(hashFile, sbOffset)
		if err != nil {
			return err
		}

		if err := adoptParamsFromSuperblock(params, sb, sbOffset); err != nil {
			return err
		}
	}

	vh := NewCryptHash(
		params.HashName,
		params.DataBlockSize, params.HashBlockSize,
		params.DataBlocks,
		params.HashType,
		params.Salt,
		params.HashAreaOffset,
		dataDevice, hashDevice,
		rootHash,
	)

	if err := validateParams(params, vh.hashFunc.Size()); err != nil {
		return err
	}

	return vh.CreateOrVerifyHashTree(true)
}

func Create(params *Params, dataDevice, hashDevice string) ([]byte, error) {
	if params == nil {
		return nil, errors.New("verity: nil params")
	}

	var sbOffset uint64
	if !params.NoSuperblock {
		sb, err := buildSuperblockFromParams(params)
		if err != nil {
			return nil, err
		}

		openFlags := os.O_RDWR | os.O_CREATE
		if dataDevice == hashDevice {
			sbOffset = params.HashAreaOffset
			params.HashAreaOffset = sbOffset + utils.AlignUp(uint64(SuperblockSize), uint64(params.HashBlockSize))
		} else {
			openFlags |= os.O_TRUNC
			sbOffset = 0
		}

		hashFile, err := os.OpenFile(hashDevice, openFlags, 0644)
		if err != nil {
			return nil, fmt.Errorf("cannot open or create hash device: %w", err)
		}
		defer hashFile.Close()

		if err := sb.WriteSuperblock(hashFile, sbOffset); err != nil {
			return nil, err
		}
	}

	vh := NewCryptHash(
		params.HashName,
		params.DataBlockSize, params.HashBlockSize,
		params.DataBlocks,
		params.HashType,
		params.Salt,
		params.HashAreaOffset,
		dataDevice, hashDevice,
		nil,
	)

	if err := validateParams(params, vh.hashFunc.Size()); err != nil {
		return nil, err
	}

	if err := vh.CreateOrVerifyHashTree(false); err != nil {
		return nil, err
	}

	return vh.RootHash(), nil
}

func VerifyBlock(params *Params, hashName string, data, salt, expectedHash []byte) error {
	vh := &CryptHash{
		hashType: params.HashType,
		hashFunc: func() crypto.Hash {
			hashMap := map[string]crypto.Hash{
				"sha256": crypto.SHA256,
				"sha512": crypto.SHA512,
				"sha1":   crypto.SHA1,
			}
			if h, ok := hashMap[hashName]; ok && h.Available() {
				return h
			}
			return crypto.SHA256
		}(),
	}

	calculatedHash, err := vh.verifyHashBlock(data, salt)
	if err != nil {
		return err
	}

	if !bytes.Equal(calculatedHash, expectedHash) {
		return fmt.Errorf("block hash mismatch")
	}

	return nil
}

func InitParams(params *Params, dataLoop, hashLoop string) error {
	if params == nil {
		return errors.New("verity: nil params")
	}

	if !params.NoSuperblock {
		f, err := os.OpenFile(hashLoop, os.O_RDONLY, 0)
		if err != nil {
			return fmt.Errorf("open hash device: %w", err)
		}
		defer f.Close()

		sbOffset := uint64(0)
		if dataLoop == hashLoop && params.HashAreaOffset > 0 {
			sbOffset = params.HashAreaOffset
		}

		sb, err := ReadSuperblock(f, sbOffset)
		if err != nil {
			return fmt.Errorf("device is not a valid VERITY device: %w", err)
		}

		if err := adoptParamsFromSuperblock(params, sb, sbOffset); err != nil {
			return fmt.Errorf("failed to adopt params from superblock: %w", err)
		}
	} else {
		if params.HashAreaOffset%uint64(params.HashBlockSize) != 0 {
			return fmt.Errorf("hash offset %d must be aligned to hash block size %d", params.HashAreaOffset, params.HashBlockSize)
		}

		if params.DataBlocks == 0 {
			size, err := utils.GetBlockOrFileSize(dataLoop)
			if err != nil {
				return fmt.Errorf("determine data device size: %w", err)
			}
			if size%int64(params.DataBlockSize) != 0 {
				return fmt.Errorf("data size %d not multiple of data block size %d", size, params.DataBlockSize)
			}
			params.DataBlocks = uint64(size / int64(params.DataBlockSize))
		}
	}

	return nil
}

func DumpDevice(hashPath string) (string, error) {
	hashFile, err := os.Open(hashPath)
	if err != nil {
		return "", fmt.Errorf("cannot open hash device: %w", err)
	}
	defer hashFile.Close()

	params := &Params{}
	superblock, err := ReadSuperblock(hashFile, 0)
	if err != nil {
		return "", fmt.Errorf("failed to read superblock: %w", err)
	}

	if err := adoptParamsFromSuperblock(params, superblock, 0); err != nil {
		return "", fmt.Errorf("failed to adopt params from superblock: %w", err)
	}

	uuidStr, err := superblock.UUIDString()
	if err != nil {
		return "", fmt.Errorf("failed to get UUID string: %w", err)
	}

	digestSize := utils.SelectHashSize(params.HashName)
	if digestSize <= 0 {
		return "", fmt.Errorf("unsupported hash algorithm: %s", params.HashName)
	}

	hashPerBlock := params.HashBlockSize / uint32(digestSize)
	hashBlocks := (params.DataBlocks + uint64(hashPerBlock) - 1) / uint64(hashPerBlock)

	deviceSizeBytes := params.HashAreaOffset + hashBlocks*uint64(params.HashBlockSize)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\nVERITY header information for %s\n", hashPath))
	sb.WriteString(fmt.Sprintf("UUID:            \t%s\n", uuidStr))
	sb.WriteString(fmt.Sprintf("Hash type:       \t%d\n", params.HashType))
	sb.WriteString(fmt.Sprintf("Data blocks:     \t%d\n", params.DataBlocks))
	sb.WriteString(fmt.Sprintf("Data block size: \t%d\n", params.DataBlockSize))
	sb.WriteString(fmt.Sprintf("Hash blocks:     \t%d\n", hashBlocks))
	sb.WriteString(fmt.Sprintf("Hash block size: \t%d\n", params.HashBlockSize))
	sb.WriteString(fmt.Sprintf("Hash algorithm:  \t%s\n", params.HashName))

	sb.WriteString("Salt:            \t")
	if params.SaltSize > 0 {
		sb.WriteString(fmt.Sprintf("%x\n", params.Salt[:params.SaltSize]))
	} else {
		sb.WriteString("-\n")
	}

	sb.WriteString(fmt.Sprintf("Hash device size: \t%d [bytes]\n", deviceSizeBytes))

	return sb.String(), nil
}

func GetHashTreeSize(params *Params) (uint64, error) {
	if params == nil {
		return 0, errors.New("verity: nil params")
	}

	if params.DataBlocks == 0 {
		return 0, errors.New("data blocks must be greater than 0")
	}

	vh := NewCryptHash(
		params.HashName,
		params.DataBlockSize, params.HashBlockSize,
		params.DataBlocks,
		params.HashType,
		nil,
		0,
		"", "",
		nil,
	)

	return vh.GetHashTreeSize()
}

func Open(params *Params, name, dataDevice, hashDevice string, rootHash []byte, signatureFile string, flags []string) (string, error) {
	var keyDesc string
	var keyID keyring.KeySerial

	if signatureFile != "" {
		if err := keyring.CheckKeyringSupport(); err != nil {
			return "", fmt.Errorf("signature verification requires kernel keyring support: %w", err)
		}
		if err := dm.CheckVeritySignatureSupport(); err != nil {
			return "", fmt.Errorf("failed to check dm-verity signature support: %w", err)
		}
	}

	if err := InitParams(params, dataDevice, hashDevice); err != nil {
		return "", fmt.Errorf("InitParams failed: %w", err)
	}

	if err := utils.ValidateRootHashSize(rootHash, params.HashName); err != nil {
		return "", err
	}

	if signatureFile != "" {
		signatureData, err := os.ReadFile(signatureFile)
		if err != nil {
			return "", fmt.Errorf("failed to read signature file: %w", err)
		}

		uuidStr := ""
		if params.UUID != ([16]byte{}) {
			uuidStr = fmt.Sprintf("%x-%x-%x-%x-%x",
				params.UUID[0:4], params.UUID[4:6], params.UUID[6:8],
				params.UUID[8:10], params.UUID[10:16])
		}

		if uuidStr != "" {
			keyDesc = fmt.Sprintf("cryptsetup:%s-%s", uuidStr, name)
		} else {
			keyDesc = fmt.Sprintf("cryptsetup:%s", name)
		}

		keyID, err = keyring.AddKeyToThreadKeyring("user", keyDesc, signatureData)
		if err != nil {
			return "", fmt.Errorf("failed to load signature into keyring: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Loaded signature into thread keyring (key ID: %d, description: %s)\n", keyID, keyDesc)

		defer func() {
			if err := keyring.UnlinkKeyFromThreadKeyring(keyID); err != nil {
				log.Printf("Warning: failed to unlink key from keyring: %v", err)
			}
		}()
	}

	openArgs := dm.OpenArgs{
		Version:            params.HashType,
		DataDevice:         dataDevice,
		HashDevice:         hashDevice,
		DataBlockSize:      params.DataBlockSize,
		HashBlockSize:      params.HashBlockSize,
		DataBlocks:         params.DataBlocks,
		HashName:           params.HashName,
		RootDigest:         rootHash,
		Salt:               params.Salt,
		HashStartBytes:     params.HashAreaOffset,
		Flags:              flags,
		RootHashSigKeyDesc: keyDesc,
	}

	targetParams, err := dm.BuildTargetParams(openArgs)
	if err != nil {
		return "", err
	}

	lengthSectors := params.DataBlocks * uint64(params.DataBlockSize/512)

	c, err := dm.Open()
	if err != nil {
		return "", err
	}
	defer c.Close()

	created := false
	defer func() {
		if !created {
			_ = c.RemoveDevice(name)
		}
	}()

	if _, err := c.CreateDevice(name); err != nil {
		return "", err
	}

	target := dm.Target{
		SectorStart: 0,
		Length:      lengthSectors,
		Type:        "verity",
		Params:      targetParams,
	}

	if err := c.LoadTable(name, []dm.Target{target}); err != nil {
		_ = c.RemoveDevice(name)
		return "", fmt.Errorf("load table: %w", err)
	}

	if err := c.SuspendDevice(name, false); err != nil {
		_ = c.RemoveDevice(name)
		if signatureFile != "" && errors.Is(err, unix.EKEYREJECTED) {
			return "", fmt.Errorf("signature verification failed: key rejected by kernel (check trusted keyring)")
		}
		return "", fmt.Errorf("resume device: %w", err)
	}

	created = true
	devPath := "/dev/mapper/" + name

	for i := 0; i < 50; i++ {
		if _, err := os.Stat(devPath); err == nil {
			return devPath, nil
		}
		time.Sleep(20 * time.Millisecond)
	}

	return devPath, nil
}

func Close(name string) error {
	c, err := dm.Open()
	if err != nil {
		return fmt.Errorf("open dm control: %w", err)
	}
	defer c.Close()

	_, err = c.DeviceStatus(name)
	if err != nil {
		return fmt.Errorf("device '%s' not found or inaccessible: %w", name, err)
	}

	if err := c.RemoveDevice(name); err != nil {
		return fmt.Errorf("remove device: %w", err)
	}

	return nil
}

func Check(deviceName string, expectedRootHash []byte) bool {
	c, err := dm.Open()
	if err != nil {
		return false
	}
	defer c.Close()

	devStatus, err := c.DeviceStatus(deviceName)
	if err != nil || !devStatus.ActivePresent {
		return false
	}

	statusFlag, err := c.TableStatus(deviceName, false)
	if err != nil {
		return false
	}

	// Check if the device is in verified state (status contains "V")
	if !strings.Contains(statusFlag, "V") {
		return false
	}

	if len(expectedRootHash) > 0 {
		tableParams, err := c.TableStatus(deviceName, true)
		if err != nil {
			return false
		}

		expectedHex := fmt.Sprintf("%x", expectedRootHash)

		if !strings.Contains(tableParams, expectedHex) {
			return false
		}
	}

	return true
}
