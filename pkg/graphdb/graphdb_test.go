package graphdb

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"testing"

	"github.com/go-check/check"
	_ "github.com/mattn/go-sqlite3"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func newTestDb(c *check.C) (*Database, string) {
	p := path.Join(os.TempDir(), "sqlite.db")
	conn, err := sql.Open("sqlite3", p)
	db, err := NewDatabase(conn)
	if err != nil {
		c.Fatal(err)
	}
	return db, p
}

func destroyTestDb(dbPath string) {
	os.Remove(dbPath)
}

func (s *DockerSuite) TestNewDatabase(c *check.C) {
	db, dbpath := newTestDb(c)
	if db == nil {
		c.Fatal("Database should not be nil")
	}
	db.Close()
	defer destroyTestDb(dbpath)
}

func (s *DockerSuite) TestCreateRootEntity(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)
	root := db.RootEntity()
	if root == nil {
		c.Fatal("Root entity should not be nil")
	}
}

func (s *DockerSuite) TestGetRootEntity(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	e := db.Get("/")
	if e == nil {
		c.Fatal("Entity should not be nil")
	}
	if e.ID() != "0" {
		c.Fatalf("Entity id should be 0, got %s", e.ID())
	}
}

func (s *DockerSuite) TestSetEntityWithDifferentName(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	db.Set("/test", "1")
	if _, err := db.Set("/other", "1"); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestSetDuplicateEntity(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	if _, err := db.Set("/foo", "42"); err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/foo", "43"); err == nil {
		c.Fatalf("Creating an entry with a duplicate path did not cause an error")
	}
}

func (s *DockerSuite) TestCreateChild(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	child, err := db.Set("/db", "1")
	if err != nil {
		c.Fatal(err)
	}
	if child == nil {
		c.Fatal("Child should not be nil")
	}
	if child.ID() != "1" {
		c.Fail()
	}
}

func (s *DockerSuite) TestParents(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	for i := 1; i < 6; i++ {
		a := strconv.Itoa(i)
		if _, err := db.Set("/"+a, a); err != nil {
			c.Fatal(err)
		}
	}

	for i := 6; i < 11; i++ {
		a := strconv.Itoa(i)
		p := strconv.Itoa(i - 5)

		key := fmt.Sprintf("/%s/%s", p, a)

		if _, err := db.Set(key, a); err != nil {
			c.Fatal(err)
		}

		parents, err := db.Parents(key)
		if err != nil {
			c.Fatal(err)
		}

		if len(parents) != 1 {
			c.Fatalf("Expected 1 entry for %s got %d", key, len(parents))
		}

		if parents[0] != p {
			c.Fatalf("ID %s received, %s expected", parents[0], p)
		}
	}
}

func (s *DockerSuite) TestChildren(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	str := "/"
	for i := 1; i < 6; i++ {
		a := strconv.Itoa(i)
		if _, err := db.Set(str+a, a); err != nil {
			c.Fatal(err)
		}

		str = str + a + "/"
	}

	str = "/"
	for i := 10; i < 30; i++ { // 20 entities
		a := strconv.Itoa(i)
		if _, err := db.Set(str+a, a); err != nil {
			c.Fatal(err)
		}

		str = str + a + "/"
	}
	entries, err := db.Children("/", 5)
	if err != nil {
		c.Fatal(err)
	}

	if len(entries) != 11 {
		c.Fatalf("Expect 11 entries for / got %d", len(entries))
	}

	entries, err = db.Children("/", 20)
	if err != nil {
		c.Fatal(err)
	}

	if len(entries) != 25 {
		c.Fatalf("Expect 25 entries for / got %d", len(entries))
	}
}

func (s *DockerSuite) TestListAllRootChildren(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}

	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	for i := 1; i < 6; i++ {
		a := strconv.Itoa(i)
		if _, err := db.Set("/"+a, a); err != nil {
			c.Fatal(err)
		}
	}
	entries := db.List("/", -1)
	if len(entries) != 5 {
		c.Fatalf("Expect 5 entries for / got %d", len(entries))
	}
}

func (s *DockerSuite) TestListAllSubChildren(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	_, err := db.Set("/webapp", "1")
	if err != nil {
		c.Fatal(err)
	}
	child2, err := db.Set("/db", "2")
	if err != nil {
		c.Fatal(err)
	}
	child4, err := db.Set("/logs", "4")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/db/logs", child4.ID()); err != nil {
		c.Fatal(err)
	}

	child3, err := db.Set("/sentry", "3")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/sentry", child3.ID()); err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/db", child2.ID()); err != nil {
		c.Fatal(err)
	}

	entries := db.List("/webapp", 1)
	if len(entries) != 3 {
		c.Fatalf("Expect 3 entries for / got %d", len(entries))
	}

	entries = db.List("/webapp", 0)
	if len(entries) != 2 {
		c.Fatalf("Expect 2 entries for / got %d", len(entries))
	}
}

func (s *DockerSuite) TestAddSelfAsChild(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	child, err := db.Set("/test", "1")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/test/other", child.ID()); err == nil {
		c.Fatal("Error should not be nil")
	}
}

func (s *DockerSuite) TestAddChildToNonExistentRoot(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	if _, err := db.Set("/myapp", "1"); err != nil {
		c.Fatal(err)
	}

	if _, err := db.Set("/myapp/proxy/db", "2"); err == nil {
		c.Fatal("Error should not be nil")
	}
}

func (s *DockerSuite) TestWalkAll(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)
	_, err := db.Set("/webapp", "1")
	if err != nil {
		c.Fatal(err)
	}
	child2, err := db.Set("/db", "2")
	if err != nil {
		c.Fatal(err)
	}
	child4, err := db.Set("/db/logs", "4")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/logs", child4.ID()); err != nil {
		c.Fatal(err)
	}

	child3, err := db.Set("/sentry", "3")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/sentry", child3.ID()); err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/db", child2.ID()); err != nil {
		c.Fatal(err)
	}

	child5, err := db.Set("/gograph", "5")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/same-ref-diff-name", child5.ID()); err != nil {
		c.Fatal(err)
	}

	if err := db.Walk("/", func(p string, e *Entity) error {
		c.Logf("Path: %s Entity: %s", p, e.ID())
		return nil
	}, -1); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestGetEntityByPath(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)
	_, err := db.Set("/webapp", "1")
	if err != nil {
		c.Fatal(err)
	}
	child2, err := db.Set("/db", "2")
	if err != nil {
		c.Fatal(err)
	}
	child4, err := db.Set("/logs", "4")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/db/logs", child4.ID()); err != nil {
		c.Fatal(err)
	}

	child3, err := db.Set("/sentry", "3")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/sentry", child3.ID()); err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/db", child2.ID()); err != nil {
		c.Fatal(err)
	}

	child5, err := db.Set("/gograph", "5")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/same-ref-diff-name", child5.ID()); err != nil {
		c.Fatal(err)
	}

	entity := db.Get("/webapp/db/logs")
	if entity == nil {
		c.Fatal("Entity should not be nil")
	}
	if entity.ID() != "4" {
		c.Fatalf("Expected to get entity with id 4, got %s", entity.ID())
	}
}

func (s *DockerSuite) TestEnitiesPaths(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)
	_, err := db.Set("/webapp", "1")
	if err != nil {
		c.Fatal(err)
	}
	child2, err := db.Set("/db", "2")
	if err != nil {
		c.Fatal(err)
	}
	child4, err := db.Set("/logs", "4")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/db/logs", child4.ID()); err != nil {
		c.Fatal(err)
	}

	child3, err := db.Set("/sentry", "3")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/sentry", child3.ID()); err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/db", child2.ID()); err != nil {
		c.Fatal(err)
	}

	child5, err := db.Set("/gograph", "5")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/same-ref-diff-name", child5.ID()); err != nil {
		c.Fatal(err)
	}

	out := db.List("/", -1)
	for _, p := range out.Paths() {
		c.Log(p)
	}
}

func (s *DockerSuite) TestDeleteRootEntity(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	if err := db.Delete("/"); err == nil {
		c.Fatal("Error should not be nil")
	}
}

func (s *DockerSuite) TestDeleteEntity(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)
	_, err := db.Set("/webapp", "1")
	if err != nil {
		c.Fatal(err)
	}
	child2, err := db.Set("/db", "2")
	if err != nil {
		c.Fatal(err)
	}
	child4, err := db.Set("/logs", "4")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/db/logs", child4.ID()); err != nil {
		c.Fatal(err)
	}

	child3, err := db.Set("/sentry", "3")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/sentry", child3.ID()); err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/db", child2.ID()); err != nil {
		c.Fatal(err)
	}

	child5, err := db.Set("/gograph", "5")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := db.Set("/webapp/same-ref-diff-name", child5.ID()); err != nil {
		c.Fatal(err)
	}

	if err := db.Delete("/webapp/sentry"); err != nil {
		c.Fatal(err)
	}
	entity := db.Get("/webapp/sentry")
	if entity != nil {
		c.Fatal("Entity /webapp/sentry should be nil")
	}
}

func (s *DockerSuite) TestCountRefs(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	db.Set("/webapp", "1")

	if db.Refs("1") != 1 {
		c.Fatal("Expect reference count to be 1")
	}

	db.Set("/db", "2")
	db.Set("/webapp/db", "2")
	if db.Refs("2") != 2 {
		c.Fatal("Expect reference count to be 2")
	}
}

func (s *DockerSuite) TestPurgeId(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}

	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	db.Set("/webapp", "1")

	if refs := db.Refs("1"); refs != 1 {
		c.Fatalf("Expect reference count to be 1, got %d", refs)
	}

	db.Set("/db", "2")
	db.Set("/webapp/db", "2")

	count, err := db.Purge("2")
	if err != nil {
		c.Fatal(err)
	}
	if count != 2 {
		c.Fatalf("Expected 2 references to be removed, got %d", count)
	}
}

// Regression test https://github.com/docker/docker/issues/12334
func (s *DockerSuite) TestPurgeIdRefPaths(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	db.Set("/webapp", "1")
	db.Set("/db", "2")

	db.Set("/db/webapp", "1")

	if refs := db.Refs("1"); refs != 2 {
		c.Fatalf("Expected 2 reference for webapp, got %d", refs)
	}
	if refs := db.Refs("2"); refs != 1 {
		c.Fatalf("Expected 1 reference for db, got %d", refs)
	}

	if rp := db.RefPaths("2"); len(rp) != 1 {
		c.Fatalf("Expected 1 reference path for db, got %d", len(rp))
	}

	count, err := db.Purge("2")
	if err != nil {
		c.Fatal(err)
	}

	if count != 2 {
		c.Fatalf("Expected 2 rows to be removed, got %d", count)
	}

	if refs := db.Refs("2"); refs != 0 {
		c.Fatalf("Expected 0 reference for db, got %d", refs)
	}
	if refs := db.Refs("1"); refs != 1 {
		c.Fatalf("Expected 1 reference for webapp, got %d", refs)
	}
}

func (s *DockerSuite) TestRename(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	db.Set("/webapp", "1")

	if db.Refs("1") != 1 {
		c.Fatal("Expect reference count to be 1")
	}

	db.Set("/db", "2")
	db.Set("/webapp/db", "2")

	if db.Get("/webapp/db") == nil {
		c.Fatal("Cannot find entity at path /webapp/db")
	}

	if err := db.Rename("/webapp/db", "/webapp/newdb"); err != nil {
		c.Fatal(err)
	}
	if db.Get("/webapp/db") != nil {
		c.Fatal("Entity should not exist at /webapp/db")
	}
	if db.Get("/webapp/newdb") == nil {
		c.Fatal("Cannot find entity at path /webapp/newdb")
	}

}

func (s *DockerSuite) TestCreateMultipleNames(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}

	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	db.Set("/db", "1")
	if _, err := db.Set("/myapp", "1"); err != nil {
		c.Fatal(err)
	}

	db.Walk("/", func(p string, e *Entity) error {
		c.Logf("%s\n", p)
		return nil
	}, -1)
}

func (s *DockerSuite) TestRefPaths(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	db.Set("/webapp", "1")

	db.Set("/db", "2")
	db.Set("/webapp/db", "2")

	refs := db.RefPaths("2")
	if len(refs) != 2 {
		c.Fatalf("Expected reference count to be 2, got %d", len(refs))
	}
}

func (s *DockerSuite) TestExistsTrue(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	db.Set("/testing", "1")

	if !db.Exists("/testing") {
		c.Fatalf("/tesing should exist")
	}
}

func (s *DockerSuite) TestExistsFalse(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	db.Set("/toerhe", "1")

	if db.Exists("/testing") {
		c.Fatalf("/tesing should not exist")
	}

}

func (s *DockerSuite) TestGetNameWithTrailingSlash(c *check.C) {
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	db.Set("/todo", "1")

	e := db.Get("/todo/")
	if e == nil {
		c.Fatalf("Entity should not be nil")
	}
}

func (s *DockerSuite) TestConcurrentWrites(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	db, dbpath := newTestDb(c)
	defer destroyTestDb(dbpath)

	errs := make(chan error, 2)

	save := func(name string, id string) {
		if _, err := db.Set(fmt.Sprintf("/%s", name), id); err != nil {
			errs <- err
		}
		errs <- nil
	}
	purge := func(id string) {
		if _, err := db.Purge(id); err != nil {
			errs <- err
		}
		errs <- nil
	}

	save("/1", "1")

	go purge("1")
	go save("/2", "2")

	any := false
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			any = true
			c.Log(err)
		}
	}
	if any {
		c.Fail()
	}
}
