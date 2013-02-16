package fs

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/coopernurse/gorp"
	"os"
	"io"
	"path"
	"github.com/dotcloud/docker/future"
)

type Store struct {
	Root	string
	db	*sql.DB
	orm	*gorp.DbMap
}

type Archive io.Reader

func New(root string) (*Store, error) {
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
	if err := orm.CreateTables(); err != nil {
		return nil, err
	}
	return &Store{
		Root: root,
		db: db,
		orm: orm,
	}, nil
}

func (store *Store) imageList(src []interface{}) ([]*Image) {
	var images []*Image
	for _, i := range src {
		img := i.(*Image)
		img.store = store
		images = append(images, img)
	}
	return images
}

func (store *Store) Images() ([]*Image, error) {
	images , err := store.orm.Select(Image{}, "select * from images")
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

func (store *Store) Get(id string) (*Image, error) {
	images, err := store.orm.Select(Image{}, "select * from images where Id=?", id)
	if err != nil {
		return nil, err
	}
	if len(images) < 1 {
		return nil, os.ErrNotExist
	}
	return images[0].(*Image), nil
}

func (store *Store) Create(layer Archive, parent *Image, pth, comment string) (*Image, error) {
	// FIXME: actually do something with the layer...
	img := &Image{
		Id :		future.RandomId(),
		Comment:	comment,
		store:		store,
	}
	path := &Path{
		Path:		path.Clean(pth),
		Image:		img.Id,
	}
	trans, err := store.orm.Begin()
	if err != nil {
		return nil, err
	}
	if err := trans.Insert(img); err != nil {
		return nil, err
	}
	if err := trans.Insert(path); err != nil {
		return nil, err
	}
	if err := trans.Commit(); err != nil {
		return nil, err
	}
	return img, nil
}

func (store *Store) Register(image *Image, pth string) error {
	// FIXME: import layer
	trans, err := store.orm.Begin()
	if err != nil {
		return err
	}
	trans.Insert(image)
	trans.Insert(&Path{Path: pth, Image: image.Id})
	return trans.Commit()
}




type Image struct {
	Id		string
	Parent		string
	Comment		string
	store		*Store	`db:"-"`
}


func (image *Image) Copy(pth string) (*Image, error) {
	if err := image.store.orm.Insert(&Path{Path: pth, Image: image.Id}); err != nil {
		return nil, err
	}
	return image, nil
}


type Path struct {
	Path	string
	Image	string
}
