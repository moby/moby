package agent

import (
	"github.com/boltdb/bolt"
	"github.com/docker/swarmkit/api"
	"github.com/gogo/protobuf/proto"
)

// Layout:
//
//  bucket(v1.tasks.<id>) ->
//			data (task protobuf)
//			status (task status protobuf)
//			assigned (key present)
var (
	bucketKeyStorageVersion = []byte("v1")
	bucketKeyTasks          = []byte("tasks")
	bucketKeyAssigned       = []byte("assigned")
	bucketKeyData           = []byte("data")
	bucketKeyStatus         = []byte("status")
)

// InitDB prepares a database for writing task data.
//
// Proper buckets will be created if they don't already exist.
func InitDB(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		_, err := createBucketIfNotExists(tx, bucketKeyStorageVersion, bucketKeyTasks)
		return err
	})
}

// GetTask retrieves the task with id from the datastore.
func GetTask(tx *bolt.Tx, id string) (*api.Task, error) {
	var t api.Task

	if err := withTaskBucket(tx, id, func(bkt *bolt.Bucket) error {
		p := bkt.Get([]byte("data"))
		if p == nil {
			return errTaskUnknown
		}

		return proto.Unmarshal(p, &t)
	}); err != nil {
		return nil, err
	}

	return &t, nil
}

// WalkTasks walks all tasks in the datastore.
func WalkTasks(tx *bolt.Tx, fn func(task *api.Task) error) error {
	bkt := getTasksBucket(tx)
	if bkt == nil {
		return nil
	}

	return bkt.ForEach(func(k, v []byte) error {
		tbkt := bkt.Bucket(k)

		p := tbkt.Get(bucketKeyData)
		var t api.Task
		if err := proto.Unmarshal(p, &t); err != nil {
			return err
		}

		return fn(&t)
	})
}

// TaskAssigned returns true if the task is assigned to the node.
func TaskAssigned(tx *bolt.Tx, id string) bool {
	bkt := getTaskBucket(tx, id)
	if bkt == nil {
		return false
	}

	return len(bkt.Get(bucketKeyAssigned)) > 0
}

// GetTaskStatus returns the current status for the task.
func GetTaskStatus(tx *bolt.Tx, id string) (*api.TaskStatus, error) {
	var ts api.TaskStatus
	if err := withTaskBucket(tx, id, func(bkt *bolt.Bucket) error {
		p := bkt.Get(bucketKeyStatus)
		if p == nil {
			return errTaskUnknown
		}

		return proto.Unmarshal(p, &ts)
	}); err != nil {
		return nil, err
	}

	return &ts, nil
}

// WalkTaskStatus calls fn for the status of each task.
func WalkTaskStatus(tx *bolt.Tx, fn func(id string, status *api.TaskStatus) error) error {
	bkt := getTasksBucket(tx)
	if bkt == nil {
		return nil
	}

	return bkt.ForEach(func(k, v []byte) error {
		tbkt := bkt.Bucket(k)

		p := tbkt.Get(bucketKeyStatus)
		var ts api.TaskStatus
		if err := proto.Unmarshal(p, &ts); err != nil {
			return err
		}

		return fn(string(k), &ts)
	})
}

// PutTask places the task into the database.
func PutTask(tx *bolt.Tx, task *api.Task) error {
	return withCreateTaskBucketIfNotExists(tx, task.ID, func(bkt *bolt.Bucket) error {
		taskCopy := *task
		taskCopy.Status = api.TaskStatus{} // blank out the status.

		p, err := proto.Marshal(&taskCopy)
		if err != nil {
			return err
		}
		return bkt.Put(bucketKeyData, p)
	})
}

// PutTaskStatus updates the status for the task with id.
func PutTaskStatus(tx *bolt.Tx, id string, status *api.TaskStatus) error {
	return withCreateTaskBucketIfNotExists(tx, id, func(bkt *bolt.Bucket) error {
		p, err := proto.Marshal(status)
		if err != nil {
			return err
		}
		return bkt.Put([]byte("status"), p)
	})
}

// DeleteTask completely removes the task from the database.
func DeleteTask(tx *bolt.Tx, id string) error {
	bkt := getTasksBucket(tx)
	if bkt == nil {
		return nil
	}

	return bkt.DeleteBucket([]byte(id))
}

// SetTaskAssignment sets the current assignment state.
func SetTaskAssignment(tx *bolt.Tx, id string, assigned bool) error {
	return withTaskBucket(tx, id, func(bkt *bolt.Bucket) error {
		if assigned {
			return bkt.Put([]byte("assigned"), []byte{0xFF})
		}
		return bkt.Delete([]byte("assigned"))
	})
}

func createBucketIfNotExists(tx *bolt.Tx, keys ...[]byte) (*bolt.Bucket, error) {
	bkt, err := tx.CreateBucketIfNotExists(keys[0])
	if err != nil {
		return nil, err
	}

	for _, key := range keys[1:] {
		bkt, err = bkt.CreateBucketIfNotExists(key)
		if err != nil {
			return nil, err
		}
	}

	return bkt, nil
}

func withCreateTaskBucketIfNotExists(tx *bolt.Tx, id string, fn func(bkt *bolt.Bucket) error) error {
	bkt, err := createBucketIfNotExists(tx, bucketKeyStorageVersion, bucketKeyTasks, []byte(id))
	if err != nil {
		return err
	}

	return fn(bkt)
}

func withTaskBucket(tx *bolt.Tx, id string, fn func(bkt *bolt.Bucket) error) error {
	bkt := getTaskBucket(tx, id)
	if bkt == nil {
		return errTaskUnknown
	}

	return fn(bkt)
}

func getTaskBucket(tx *bolt.Tx, id string) *bolt.Bucket {
	return getBucket(tx, bucketKeyStorageVersion, bucketKeyTasks, []byte(id))
}

func getTasksBucket(tx *bolt.Tx) *bolt.Bucket {
	return getBucket(tx, bucketKeyStorageVersion, bucketKeyTasks)
}

func getBucket(tx *bolt.Tx, keys ...[]byte) *bolt.Bucket {
	bkt := tx.Bucket(keys[0])

	for _, key := range keys[1:] {
		if bkt == nil {
			break
		}
		bkt = bkt.Bucket(key)
	}

	return bkt
}
