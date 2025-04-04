package types // import "github.com/docker/docker/api/types"

import (
	"bytes"
	"encoding/json"
)

func (du *DiskUsage) UnmarshalJSON(data []byte) error {
	var v struct {
		LayersSize  int64
		Images      json.RawMessage
		Containers  json.RawMessage
		Volumes     json.RawMessage
		BuildCache  json.RawMessage
		BuilderSize int64
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	du.Images = nil
	if len(v.Images) > 0 && !bytes.Equal(v.Images, []byte("null")) {
		du.Images = &ImageDiskUsage{}
		if v.Images[0] == '[' {
			if err := json.Unmarshal(v.Images, &du.Images.Items); err != nil {
				return err
			}
		} else if err := json.Unmarshal(v.Images, du.Images); err != nil {
			return err
		}
	}

	du.Containers = nil
	if len(v.Containers) > 0 && !bytes.Equal(v.Images, []byte("null")) {
		du.Containers = &ContainerDiskUsage{}
		if v.Containers[0] == '[' {
			if err := json.Unmarshal(v.Containers, &du.Containers.Items); err != nil {
				return err
			}
		} else if err := json.Unmarshal(v.Containers, du.Containers); err != nil {
			return err
		}
	}

	du.Volumes = nil
	if len(v.Volumes) > 0 && !bytes.Equal(v.Volumes, []byte("null")) {
		du.Volumes = &VolumeDiskUsage{}
		if v.Volumes[0] == '[' {
			if err := json.Unmarshal(v.Volumes, &du.Volumes.Items); err != nil {
				return err
			}
		} else if err := json.Unmarshal(v.Volumes, du.Volumes); err != nil {
			return err
		}
	}

	du.BuildCache = nil
	if len(v.BuildCache) > 0 && !bytes.Equal(v.BuildCache, []byte("null")) {
		du.BuildCache = &BuildCacheDiskUsage{}
		if v.BuildCache[0] == '[' {
			if err := json.Unmarshal(v.BuildCache, &du.BuildCache.Items); err != nil {
				return err
			}
		} else if err := json.Unmarshal(v.BuildCache, du.BuildCache); err != nil {
			return err
		}
	}
	return nil
}
