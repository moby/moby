package blobstore

import (
	"os"
)

// Ref increments the reference count for the blob with the given digest.
func (ls *localStore) Ref(digest, refID string) (d Descriptor, err error) {
	ls.Lock()
	defer ls.Unlock()

	// Avoid the (type, nil) interface issue.
	d, blobErr := ls.ref(digest, refID)
	if blobErr != nil {
		return nil, blobErr
	}

	return
}

// Deref decrements the reference count for the blob with the given digest.
// If the reference count reaches 0, the blob will be removed from the
// store.
func (ls *localStore) Deref(digest, refID string) error {
	ls.Lock()
	defer ls.Unlock()

	// Avoid the (type, nil) interface issue.
	blobErr := ls.deref(digest, refID)
	if blobErr != nil {
		return blobErr
	}

	return nil
}

// ref is the unexported version of Ref which does not acquire the store lock
// before incrementing a blob reference count.
func (ls *localStore) ref(digest, refID string) (Descriptor, *storeError) {
	info, err := ls.getBlobInfo(digest)
	if err != nil {
		return nil, err
	}

	refSet := make(map[string]struct{}, len(info.References)+1)
	for _, refID := range info.References {
		refSet[refID] = struct{}{}
	}

	refSet[refID] = struct{}{}

	references := make([]string, 0, len(refSet))
	for refID := range refSet {
		references = append(references, refID)
	}

	info.References = references

	if err = ls.putBlobInfo(info); err != nil {
		return nil, err
	}

	return newDescriptor(info), nil
}

// deref is the unexported version of Deref which does not acquire the store
// lock before decrementing the blob reference count.
func (ls *localStore) deref(digest, refID string) *storeError {
	info, err := ls.getBlobInfo(digest)
	if err != nil {
		return err
	}

	refSet := make(map[string]struct{}, len(info.References))
	for _, refID := range info.References {
		refSet[refID] = struct{}{}
	}

	delete(refSet, refID)

	if len(refSet) > 0 {
		references := make([]string, 0, len(refSet))
		for refID := range refSet {
			references = append(references, refID)
		}

		info.References = references
		return ls.putBlobInfo(info)
	}

	blobDirname := ls.blobDirname(digest)
	if e := os.RemoveAll(blobDirname); e != nil {
		return newError(errCodeCannotRemoveBlob, e.Error())
	}

	return nil
}
