package cache

import (
	"time"

	"github.com/boltdb/bolt"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/pkg/errors"
)

const sizeUnknown int64 = -1
const keySize = "snapshot.size"
const keyEqualMutable = "cache.equalMutable"
const keyCachePolicy = "cache.cachePolicy"
const keyDescription = "cache.description"
const keyCreatedAt = "cache.createdAt"
const keyLastUsedAt = "cache.lastUsedAt"
const keyUsageCount = "cache.usageCount"

const keyDeleted = "cache.deleted"

func setDeleted(si *metadata.StorageItem) error {
	v, err := metadata.NewValue(true)
	if err != nil {
		return errors.Wrap(err, "failed to create size value")
	}
	si.Update(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyDeleted, v)
	})
	return nil
}

func getDeleted(si *metadata.StorageItem) bool {
	v := si.Get(keyDeleted)
	if v == nil {
		return false
	}
	var deleted bool
	if err := v.Unmarshal(&deleted); err != nil {
		return false
	}
	return deleted
}

func setSize(si *metadata.StorageItem, s int64) error {
	v, err := metadata.NewValue(s)
	if err != nil {
		return errors.Wrap(err, "failed to create size value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keySize, v)
	})
	return nil
}

func getSize(si *metadata.StorageItem) int64 {
	v := si.Get(keySize)
	if v == nil {
		return sizeUnknown
	}
	var size int64
	if err := v.Unmarshal(&size); err != nil {
		return sizeUnknown
	}
	return size
}

func getEqualMutable(si *metadata.StorageItem) string {
	v := si.Get(keyEqualMutable)
	if v == nil {
		return ""
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func setEqualMutable(si *metadata.StorageItem, s string) error {
	v, err := metadata.NewValue(s)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s meta value", keyEqualMutable)
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyEqualMutable, v)
	})
	return nil
}

func clearEqualMutable(si *metadata.StorageItem) error {
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyEqualMutable, nil)
	})
	return nil
}

func queueCachePolicy(si *metadata.StorageItem, p cachePolicy) error {
	v, err := metadata.NewValue(p)
	if err != nil {
		return errors.Wrap(err, "failed to create cachePolicy value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyCachePolicy, v)
	})
	return nil
}

func getCachePolicy(si *metadata.StorageItem) cachePolicy {
	v := si.Get(keyCachePolicy)
	if v == nil {
		return cachePolicyDefault
	}
	var p cachePolicy
	if err := v.Unmarshal(&p); err != nil {
		return cachePolicyDefault
	}
	return p
}

func queueDescription(si *metadata.StorageItem, descr string) error {
	v, err := metadata.NewValue(descr)
	if err != nil {
		return errors.Wrap(err, "failed to create description value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyDescription, v)
	})
	return nil
}

func GetDescription(si *metadata.StorageItem) string {
	v := si.Get(keyDescription)
	if v == nil {
		return ""
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func queueCreatedAt(si *metadata.StorageItem, tm time.Time) error {
	v, err := metadata.NewValue(tm.UnixNano())
	if err != nil {
		return errors.Wrap(err, "failed to create createdAt value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyCreatedAt, v)
	})
	return nil
}

func GetCreatedAt(si *metadata.StorageItem) time.Time {
	v := si.Get(keyCreatedAt)
	if v == nil {
		return time.Time{}
	}
	var tm int64
	if err := v.Unmarshal(&tm); err != nil {
		return time.Time{}
	}
	return time.Unix(tm/1e9, tm%1e9)
}

func getLastUsed(si *metadata.StorageItem) (int, *time.Time) {
	v := si.Get(keyUsageCount)
	if v == nil {
		return 0, nil
	}
	var usageCount int
	if err := v.Unmarshal(&usageCount); err != nil {
		return 0, nil
	}
	v = si.Get(keyLastUsedAt)
	if v == nil {
		return usageCount, nil
	}
	var lastUsedTs int64
	if err := v.Unmarshal(&lastUsedTs); err != nil || lastUsedTs == 0 {
		return usageCount, nil
	}
	tm := time.Unix(lastUsedTs/1e9, lastUsedTs%1e9)
	return usageCount, &tm
}

func updateLastUsed(si *metadata.StorageItem) error {
	count, _ := getLastUsed(si)
	count++

	v, err := metadata.NewValue(count)
	if err != nil {
		return errors.Wrap(err, "failed to create usageCount value")
	}
	v2, err := metadata.NewValue(time.Now().UnixNano())
	if err != nil {
		return errors.Wrap(err, "failed to create lastUsedAt value")
	}
	return si.Update(func(b *bolt.Bucket) error {
		if err := si.SetValue(b, keyUsageCount, v); err != nil {
			return err
		}
		return si.SetValue(b, keyLastUsedAt, v2)
	})
}
