package cache

import (
	"time"

	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/client"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

const sizeUnknown int64 = -1
const keySize = "snapshot.size"
const keyEqualMutable = "cache.equalMutable"
const keyCachePolicy = "cache.cachePolicy"
const keyDescription = "cache.description"
const keyCreatedAt = "cache.createdAt"
const keyLastUsedAt = "cache.lastUsedAt"
const keyUsageCount = "cache.usageCount"
const keyLayerType = "cache.layerType"
const keyRecordType = "cache.recordType"
const keyCommitted = "snapshot.committed"
const keyParent = "cache.parent"
const keyDiffID = "cache.diffID"
const keyChainID = "cache.chainID"
const keyBlobChainID = "cache.blobChainID"
const keyBlob = "cache.blob"
const keySnapshot = "cache.snapshot"
const keyBlobOnly = "cache.blobonly"
const keyMediaType = "cache.mediatype"

const keyDeleted = "cache.deleted"

func queueDiffID(si *metadata.StorageItem, str string) error {
	if str == "" {
		return nil
	}
	v, err := metadata.NewValue(str)
	if err != nil {
		return errors.Wrap(err, "failed to create diffID value")
	}
	si.Update(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyDiffID, v)
	})
	return nil
}

func getMediaType(si *metadata.StorageItem) string {
	v := si.Get(keyMediaType)
	if v == nil {
		return si.ID()
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func queueMediaType(si *metadata.StorageItem, str string) error {
	if str == "" {
		return nil
	}
	v, err := metadata.NewValue(str)
	if err != nil {
		return errors.Wrap(err, "failed to create mediaType value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyMediaType, v)
	})
	return nil
}

func getSnapshotID(si *metadata.StorageItem) string {
	v := si.Get(keySnapshot)
	if v == nil {
		return si.ID()
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func queueSnapshotID(si *metadata.StorageItem, str string) error {
	if str == "" {
		return nil
	}
	v, err := metadata.NewValue(str)
	if err != nil {
		return errors.Wrap(err, "failed to create chainID value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keySnapshot, v)
	})
	return nil
}

func getDiffID(si *metadata.StorageItem) string {
	v := si.Get(keyDiffID)
	if v == nil {
		return ""
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func queueChainID(si *metadata.StorageItem, str string) error {
	if str == "" {
		return nil
	}
	v, err := metadata.NewValue(str)
	if err != nil {
		return errors.Wrap(err, "failed to create chainID value")
	}
	v.Index = "chainid:" + str
	si.Update(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyChainID, v)
	})
	return nil
}

func getBlobChainID(si *metadata.StorageItem) string {
	v := si.Get(keyBlobChainID)
	if v == nil {
		return ""
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func queueBlobChainID(si *metadata.StorageItem, str string) error {
	if str == "" {
		return nil
	}
	v, err := metadata.NewValue(str)
	if err != nil {
		return errors.Wrap(err, "failed to create chainID value")
	}
	v.Index = "blobchainid:" + str
	si.Update(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyBlobChainID, v)
	})
	return nil
}

func getChainID(si *metadata.StorageItem) string {
	v := si.Get(keyChainID)
	if v == nil {
		return ""
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func queueBlob(si *metadata.StorageItem, str string) error {
	if str == "" {
		return nil
	}
	v, err := metadata.NewValue(str)
	if err != nil {
		return errors.Wrap(err, "failed to create blob value")
	}
	si.Update(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyBlob, v)
	})
	return nil
}

func getBlob(si *metadata.StorageItem) string {
	v := si.Get(keyBlob)
	if v == nil {
		return ""
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func queueBlobOnly(si *metadata.StorageItem, b bool) error {
	v, err := metadata.NewValue(b)
	if err != nil {
		return errors.Wrap(err, "failed to create blobonly value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyBlobOnly, v)
	})
	return nil
}

func getBlobOnly(si *metadata.StorageItem) bool {
	v := si.Get(keyBlobOnly)
	if v == nil {
		return false
	}
	var blobOnly bool
	if err := v.Unmarshal(&blobOnly); err != nil {
		return false
	}
	return blobOnly
}

func setDeleted(si *metadata.StorageItem) error {
	v, err := metadata.NewValue(true)
	if err != nil {
		return errors.Wrap(err, "failed to create deleted value")
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

func queueCommitted(si *metadata.StorageItem) error {
	v, err := metadata.NewValue(true)
	if err != nil {
		return errors.Wrap(err, "failed to create committed value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyCommitted, v)
	})
	return nil
}

func getCommitted(si *metadata.StorageItem) bool {
	v := si.Get(keyCommitted)
	if v == nil {
		return false
	}
	var committed bool
	if err := v.Unmarshal(&committed); err != nil {
		return false
	}
	return committed
}

func queueParent(si *metadata.StorageItem, parent string) error {
	if parent == "" {
		return nil
	}
	v, err := metadata.NewValue(parent)
	if err != nil {
		return errors.Wrap(err, "failed to create parent value")
	}
	si.Update(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyParent, v)
	})
	return nil
}

func getParent(si *metadata.StorageItem) string {
	v := si.Get(keyParent)
	if v == nil {
		return ""
	}
	var parent string
	if err := v.Unmarshal(&parent); err != nil {
		return ""
	}
	return parent
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

func SetLayerType(m withMetadata, value string) error {
	v, err := metadata.NewValue(value)
	if err != nil {
		return errors.Wrap(err, "failed to create layertype value")
	}
	m.Metadata().Queue(func(b *bolt.Bucket) error {
		return m.Metadata().SetValue(b, keyLayerType, v)
	})
	return m.Metadata().Commit()
}

func GetLayerType(m withMetadata) string {
	v := m.Metadata().Get(keyLayerType)
	if v == nil {
		return ""
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func GetRecordType(m withMetadata) client.UsageRecordType {
	v := m.Metadata().Get(keyRecordType)
	if v == nil {
		return ""
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return client.UsageRecordType(str)
}

func SetRecordType(m withMetadata, value client.UsageRecordType) error {
	if err := queueRecordType(m.Metadata(), value); err != nil {
		return err
	}
	return m.Metadata().Commit()
}

func queueRecordType(si *metadata.StorageItem, value client.UsageRecordType) error {
	v, err := metadata.NewValue(value)
	if err != nil {
		return errors.Wrap(err, "failed to create recordtype value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyRecordType, v)
	})
	return nil
}
