package changelist

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/uuid"
	"path/filepath"
)

// FileChangelist stores all the changes as files
type FileChangelist struct {
	dir string
}

// NewFileChangelist is a convenience method for returning FileChangeLists
func NewFileChangelist(dir string) (*FileChangelist, error) {
	logrus.Debug("Making dir path: ", dir)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return nil, err
	}
	return &FileChangelist{dir: dir}, nil
}

// getFileNames reads directory, filtering out child directories
func getFileNames(dirName string) ([]os.FileInfo, error) {
	var dirListing, fileInfos []os.FileInfo
	dir, err := os.Open(dirName)
	if err != nil {
		return fileInfos, err
	}
	defer dir.Close()
	dirListing, err = dir.Readdir(0)
	if err != nil {
		return fileInfos, err
	}
	for _, f := range dirListing {
		if f.IsDir() {
			continue
		}
		fileInfos = append(fileInfos, f)
	}
	sort.Sort(fileChanges(fileInfos))
	return fileInfos, nil
}

// Read a JSON formatted file from disk; convert to TUFChange struct
func unmarshalFile(dirname string, f os.FileInfo) (*TUFChange, error) {
	c := &TUFChange{}
	raw, err := ioutil.ReadFile(filepath.Join(dirname, f.Name()))
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(raw, c)
	if err != nil {
		return c, err
	}
	return c, nil
}

// List returns a list of sorted changes
func (cl FileChangelist) List() []Change {
	var changes []Change
	fileInfos, err := getFileNames(cl.dir)
	if err != nil {
		return changes
	}
	for _, f := range fileInfos {
		c, err := unmarshalFile(cl.dir, f)
		if err != nil {
			logrus.Warn(err.Error())
			continue
		}
		changes = append(changes, c)
	}
	return changes
}

// Add adds a change to the file change list
func (cl FileChangelist) Add(c Change) error {
	cJSON, err := json.Marshal(c)
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("%020d_%s.change", time.Now().UnixNano(), uuid.Generate())
	return ioutil.WriteFile(filepath.Join(cl.dir, filename), cJSON, 0644)
}

// Remove deletes the changes found at the given indices
func (cl FileChangelist) Remove(idxs []int) error {
	fileInfos, err := getFileNames(cl.dir)
	if err != nil {
		return err
	}
	remove := make(map[int]struct{})
	for _, i := range idxs {
		remove[i] = struct{}{}
	}
	for i, c := range fileInfos {
		if _, ok := remove[i]; ok {
			file := filepath.Join(cl.dir, c.Name())
			if err := os.Remove(file); err != nil {
				logrus.Errorf("could not remove change %d: %s", i, err.Error())
			}
		}
	}
	return nil
}

// Clear clears the change list
// N.B. archiving not currently implemented
func (cl FileChangelist) Clear(archive string) error {
	dir, err := os.Open(cl.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	files, err := dir.Readdir(0)
	if err != nil {
		return err
	}
	for _, f := range files {
		os.Remove(filepath.Join(cl.dir, f.Name()))
	}
	return nil
}

// Close is a no-op
func (cl FileChangelist) Close() error {
	// Nothing to do here
	return nil
}

// NewIterator creates an iterator from FileChangelist
func (cl FileChangelist) NewIterator() (ChangeIterator, error) {
	fileInfos, err := getFileNames(cl.dir)
	if err != nil {
		return &FileChangeListIterator{}, err
	}
	return &FileChangeListIterator{dirname: cl.dir, collection: fileInfos}, nil
}

// IteratorBoundsError is an Error type used by Next()
type IteratorBoundsError int

// Error implements the Error interface
func (e IteratorBoundsError) Error() string {
	return fmt.Sprintf("Iterator index (%d) out of bounds", e)
}

// FileChangeListIterator is a concrete instance of ChangeIterator
type FileChangeListIterator struct {
	index      int
	dirname    string
	collection []os.FileInfo
}

// Next returns the next Change in the FileChangeList
func (m *FileChangeListIterator) Next() (item Change, err error) {
	if m.index >= len(m.collection) {
		return nil, IteratorBoundsError(m.index)
	}
	f := m.collection[m.index]
	m.index++
	item, err = unmarshalFile(m.dirname, f)
	return
}

// HasNext indicates whether iterator is exhausted
func (m *FileChangeListIterator) HasNext() bool {
	return m.index < len(m.collection)
}

type fileChanges []os.FileInfo

// Len returns the length of a file change list
func (cs fileChanges) Len() int {
	return len(cs)
}

// Less compares the names of two different file changes
func (cs fileChanges) Less(i, j int) bool {
	return cs[i].Name() < cs[j].Name()
}

// Swap swaps the position of two file changes
func (cs fileChanges) Swap(i, j int) {
	tmp := cs[i]
	cs[i] = cs[j]
	cs[j] = tmp
}
