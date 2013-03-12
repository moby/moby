package fs

import (
	"../future"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shykes/gorp" //Forked to implement CreateTablesOpts
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"
)

type Store struct {
	Root   string
	db     *sql.DB
	orm    *gorp.DbMap
	layers *LayerStore
}

type Archive io.Reader

func New(root string) (*Store, error) {
	isNewStore := true

	if err := os.Mkdir(root, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path.Join(root, "db"))
	if err != nil {
		return nil, err
	}
	orm := &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}
	orm.AddTableWithName(Image{}, "images").SetKeys(false, "Id")
	orm.AddTableWithName(Path{}, "paths").SetKeys(false, "Path", "Image")
	orm.AddTableWithName(Mountpoint{}, "mountpoints").SetKeys(false, "Root")
	orm.AddTableWithName(Tag{}, "tags").SetKeys(false, "TagName")
	if isNewStore {
		if err := orm.CreateTablesOpts(true); err != nil {
			return nil, err
		}
	}

	layers, err := NewLayerStore(path.Join(root, "layers"))
	if err != nil {
		return nil, err
	}
	return &Store{
		Root:   root,
		db:     db,
		orm:    orm,
		layers: layers,
	}, nil
}

func (store *Store) imageList(src []interface{}) []*Image {
	var images []*Image
	for _, i := range src {
		img := i.(*Image)
		img.store = store
		images = append(images, img)
	}
	return images
}

func (store *Store) Images() ([]*Image, error) {
	images, err := store.orm.Select(Image{}, "select * from images")
	if err != nil {
		return nil, err
	}
	return store.imageList(images), nil
}

func (store *Store) Paths() ([]string, error) {
	var paths []string
	rows, err := store.db.Query("select distinct Path from paths order by Path")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func (store *Store) List(pth string) ([]*Image, error) {
	pth = path.Clean(pth)
	images, err := store.orm.Select(Image{}, "select images.* from images, paths where Path=? and paths.Image=images.Id", pth)
	if err != nil {
		return nil, err
	}
	return store.imageList(images), nil
}

func (store *Store) Find(pth string) (*Image, error) {
	pth = path.Clean(pth)
	img, err := store.Get(pth)
	if err != nil {
		return nil, err
	} else if img != nil {
		return img, nil
	}

	images, err := store.orm.Select(Image{}, "select images.* from images, paths where Path=? and paths.Image=images.Id order by images.Created desc limit 1", pth)
	if err != nil {
		return nil, err
	} else if len(images) < 1 {
		return nil, nil
	}
	img = images[0].(*Image)
	img.store = store
	return img, nil
}

func (store *Store) Get(id string) (*Image, error) {
	img, err := store.orm.Get(Image{}, id)
	if img == nil {
		return nil, err
	}
	res := img.(*Image)
	res.store = store
	return res, err
}

func (store *Store) Create(layerData Archive, parent *Image, pth, comment string) (*Image, error) {
	// FIXME: actually do something with the layer...
	img := &Image{
		Id:      future.RandomId(),
		Comment: comment,
		Created: time.Now().Unix(),
		store:   store,
	}
	if parent != nil {
		img.Parent = parent.Id
	}
	// FIXME: we shouldn't have to pass os.Stderr to AddLayer()...
	// FIXME: Archive should contain compression info. For now we only support uncompressed.
	_, err := store.layers.AddLayer(img.Id, layerData)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not add layer: %s", err))
	}
	path := &Path{
		Path:  path.Clean(pth),
		Image: img.Id,
	}
	trans, err := store.orm.Begin()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not begin transaction:", err))
	}
	if err := trans.Insert(img); err != nil {
		return nil, errors.New(fmt.Sprintf("Could not insert image info: %s", err))
	}
	if err := trans.Insert(path); err != nil {
		return nil, errors.New(fmt.Sprintf("Could not insert path info: %s", err))
	}
	if err := trans.Commit(); err != nil {
		return nil, errors.New(fmt.Sprintf("Could not commit transaction: %s", err))
	}
	return img, nil
}

func (store *Store) Register(image *Image, pth string) error {
	image.store = store
	// FIXME: import layer
	trans, err := store.orm.Begin()
	if err != nil {
		return err
	}
	trans.Insert(image)
	trans.Insert(&Path{Path: pth, Image: image.Id})
	return trans.Commit()
}

func (store *Store) Layers() []string {
	return store.layers.List()
}

type Image struct {
	Id      string
	Parent  string
	Comment string
	Created int64
	store   *Store `db:"-"`
}

func (image *Image) Copy(pth string) (*Image, error) {
	if err := image.store.orm.Insert(&Path{Path: pth, Image: image.Id}); err != nil {
		return nil, err
	}
	return image, nil
}

type Mountpoint struct {
	Image string
	Root  string
	Rw    string
	Store *Store `db:"-"`
}

func (image *Image) Mountpoint(root, rw string) (*Mountpoint, error) {
	mountpoint := &Mountpoint{
		Root:  path.Clean(root),
		Rw:    path.Clean(rw),
		Image: image.Id,
		Store: image.store,
	}
	if err := image.store.orm.Insert(mountpoint); err != nil {
		return nil, err
	}
	return mountpoint, nil
}

func (image *Image) layers() ([]string, error) {
	var list []string
	var err error
	currentImg := image
	for currentImg != nil {
		if layer := image.store.layers.Get(currentImg.Id); layer != "" {
			list = append(list, layer)
		} else {
			return list, fmt.Errorf("Layer not found for image %s", image.Id)
		}
		currentImg, err = currentImg.store.Get(currentImg.Parent)
		if err != nil {
			return list, fmt.Errorf("Error while getting parent image: %v", err)
		}
	}
	return list, nil
}

func (image *Image) Mountpoints() ([]*Mountpoint, error) {
	var mountpoints []*Mountpoint
	res, err := image.store.orm.Select(Mountpoint{}, "select * from mountpoints where Image=?", image.Id)
	if err != nil {
		return nil, err
	}
	for _, mp := range res {
		mountpoints = append(mountpoints, mp.(*Mountpoint))
	}
	return mountpoints, nil
}

func (image *Image) Mount(root, rw string) (*Mountpoint, error) {
	var mountpoint *Mountpoint
	if mp, err := image.store.FetchMountpoint(root, rw); err != nil {
		return nil, err
	} else if mp == nil {
		mountpoint, err = image.Mountpoint(root, rw)
		if err != nil {
			return nil, fmt.Errorf("Could not create mountpoint: %s", err)
		} else if mountpoint == nil {
			return nil, errors.New("No mountpoint created")
		}
	} else {
		mountpoint = mp
	}

	if err := mountpoint.createFolders(); err != nil {
		return nil, err
	}

	// FIXME: Now mount the layers
	rwBranch := fmt.Sprintf("%v=rw", mountpoint.Rw)
	roBranches := ""
	layers, err := image.layers()
	if err != nil {
		return nil, err
	}
	for _, layer := range layers {
		roBranches += fmt.Sprintf("%v=ro:", layer)
	}
	branches := fmt.Sprintf("br:%v:%v", rwBranch, roBranches)
	if err := mount("none", mountpoint.Root, "aufs", 0, branches); err != nil {
		return mountpoint, err
	}
	if !mountpoint.Mounted() {
		return mountpoint, errors.New("Mount failed")
	}

	return mountpoint, nil
}

func (mp *Mountpoint) EnsureMounted() error {
	if mp.Mounted() {
		return nil
	}
	img, err := mp.Store.Get(mp.Image)
	if err != nil {
		return err
	}

	_, err = img.Mount(mp.Root, mp.Rw)
	return err
}

func (mp *Mountpoint) createFolders() error {
	if err := os.Mkdir(mp.Root, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := os.Mkdir(mp.Rw, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func (mp *Mountpoint) Mounted() bool {
	root, err := os.Stat(mp.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	}
	parent, err := os.Stat(filepath.Join(mp.Root, ".."))
	if err != nil {
		panic(err)
	}

	rootSt := root.Sys().(*syscall.Stat_t)
	parentSt := parent.Sys().(*syscall.Stat_t)
	return rootSt.Dev != parentSt.Dev
}

func (mp *Mountpoint) Umount() error {
	if !mp.Mounted() {
		return errors.New("Mountpoint doesn't seem to be mounted")
	}
	if err := syscall.Unmount(mp.Root, 0); err != nil {
		return fmt.Errorf("Unmount syscall failed: %v", err)
	}
	if mp.Mounted() {
		return fmt.Errorf("Umount: Filesystem still mounted after calling umount(%v)", mp.Root)
	}
	// Even though we just unmounted the filesystem, AUFS will prevent deleting the mntpoint
	// for some time. We'll just keep retrying until it succeeds.
	for retries := 0; retries < 1000; retries++ {
		err := os.Remove(mp.Root)
		if err == nil {
			// rm mntpoint succeeded
			return nil
		}
		if os.IsNotExist(err) {
			// mntpoint doesn't exist anymore. Success.
			return nil
		}
		// fmt.Printf("(%v) Remove %v returned: %v\n", retries, mp.Root, err)
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("Umount: Failed to umount %v", mp.Root)

}

func (mp *Mountpoint) Deregister() error {
	if mp.Mounted() {
		return errors.New("Mountpoint is currently mounted, can't deregister")
	}

	_, err := mp.Store.orm.Delete(mp)
	return err
}

func (store *Store) FetchMountpoint(root, rw string) (*Mountpoint, error) {
	res, err := store.orm.Select(Mountpoint{}, "select * from mountpoints where Root=? and Rw=?", root, rw)
	if err != nil {
		return nil, err
	} else if len(res) < 1 || res[0] == nil {
		return nil, nil
	}

	mp := res[0].(*Mountpoint)
	mp.Store = store
	return mp, nil
}

// OpenFile opens the named file for reading.
func (mp *Mountpoint) OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	if err := mp.EnsureMounted(); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(mp.Root, path), flag, perm)
}

// ReadDir reads the directory named by dirname, relative to the Mountpoint's root,
// and returns a list of sorted directory entries
func (mp *Mountpoint) ReadDir(dirname string) ([]os.FileInfo, error) {
	if err := mp.EnsureMounted(); err != nil {
		return nil, err
	}
	return ioutil.ReadDir(filepath.Join(mp.Root, dirname))
}

func (store *Store) AddTag(imageId, tagName string) error {
	if image, err := store.Get(imageId); err != nil {
		return err
	} else if image == nil {
		return errors.New("No image with ID " + imageId)
	}

	err2 := store.orm.Insert(&Tag{
		TagName: tagName,
		Image:   imageId,
	})

	return err2
}

func (store *Store) GetByTag(tagName string) (*Image, error) {
	res, err := store.orm.Get(Tag{}, tagName)
	if err != nil {
		return nil, err
	} else if res == nil {
		return nil, errors.New("No image associated to tag \"" + tagName + "\"")
	}

	tag := res.(*Tag)

	img, err2 := store.Get(tag.Image)
	if err2 != nil {
		return nil, err2
	} else if img == nil {
		return nil, errors.New("Tag was found but image seems to be inexistent.")
	}

	return img, nil
}

type Path struct {
	Path  string
	Image string
}

type Tag struct {
	TagName string
	Image   string
}
