package gograph

import (
	"os"
	"strconv"
	"testing"
)

func newTestDb(t *testing.T) *Database {
	db, err := NewDatabase(os.TempDir(), "0")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestNewDatabase(t *testing.T) {
	db := newTestDb(t)
	if db == nil {
		t.Fatal("Datbase should not be nil")
	}
}

func TestCreateRootEnity(t *testing.T) {
	db := newTestDb(t)
	root := db.RootEntity()
	if root == nil {
		t.Fatal("Root entity should not be nil")
	}
}

func TestGetRootEntity(t *testing.T) {
	db := newTestDb(t)

	e := db.Get("/")
	if e == nil {
		t.Fatal("Entity should not be nil")
	}
	if e.ID() != "0" {
		t.Fatalf("Enity id should be 0, got %s", e.ID())
	}
}

func TestSetEntityWithDifferentName(t *testing.T) {
	db := newTestDb(t)

	db.Set("/test", "1")
	if _, err := db.Set("/other", "1"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateChild(t *testing.T) {
	db := newTestDb(t)

	child, err := db.Set("/db", "1")
	if err != nil {
		t.Fatal(err)
	}
	if child == nil {
		t.Fatal("Child should not be nil")
	}
	if child.ID() != "1" {
		t.Fail()
	}
}

func TestListAllRootChildren(t *testing.T) {
	db := newTestDb(t)

	for i := 1; i < 6; i++ {
		a := strconv.Itoa(i)
		if _, err := db.Set("/"+a, a); err != nil {
			t.Fatal(err)
		}
	}
	entries := db.List("/", -1)
	if len(entries) != 5 {
		t.Fatalf("Expect 5 entries for / got %d", len(entries))
	}
}

func TestListAllSubChildren(t *testing.T) {
	db := newTestDb(t)

	_, err := db.Set("/webapp", "1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := db.Set("/db", "2")
	if err != nil {
		t.Fatal(err)
	}
	child4, err := db.Set("/logs", "4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/db/logs", child4.ID()); err != nil {
		t.Fatal(err)
	}

	child3, err := db.Set("/sentry", "3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/sentry", child3.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/db", child2.ID()); err != nil {
		t.Fatal(err)
	}

	entries := db.List("/webapp", 1)
	if len(entries) != 3 {
		t.Fatalf("Expect 3 entries for / got %d", len(entries))
	}

	entries = db.List("/webapp", 0)
	if len(entries) != 2 {
		t.Fatalf("Expect 2 entries for / got %d", len(entries))
	}
}

func TestAddSelfAsChild(t *testing.T) {
	db := newTestDb(t)

	child, err := db.Set("/test", "1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/test/other", child.ID()); err == nil {
		t.Fatal("Error should not be nil")
	}
}

func TestAddChildToNonExistantRoot(t *testing.T) {
	db := newTestDb(t)

	if _, err := db.Set("/myapp", "1"); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Set("/myapp/proxy/db", "2"); err == nil {
		t.Fatal("Error should not be nil")
	}
}

func TestWalkAll(t *testing.T) {
	db := newTestDb(t)
	_, err := db.Set("/webapp", "1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := db.Set("/db", "2")
	if err != nil {
		t.Fatal(err)
	}
	child4, err := db.Set("/db/logs", "4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/logs", child4.ID()); err != nil {
		t.Fatal(err)
	}

	child3, err := db.Set("/sentry", "3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/sentry", child3.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/db", child2.ID()); err != nil {
		t.Fatal(err)
	}

	child5, err := db.Set("/gograph", "5")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/same-ref-diff-name", child5.ID()); err != nil {
		t.Fatal(err)
	}

	if err := db.Walk("/", func(p string, e *Entity) error {
		t.Logf("Path: %s Entity: %s", p, e.ID())
		return nil
	}, -1); err != nil {
		t.Fatal(err)
	}
}

func TestGetEntityByPath(t *testing.T) {
	db := newTestDb(t)
	_, err := db.Set("/webapp", "1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := db.Set("/db", "2")
	if err != nil {
		t.Fatal(err)
	}
	child4, err := db.Set("/logs", "4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/db/logs", child4.ID()); err != nil {
		t.Fatal(err)
	}

	child3, err := db.Set("/sentry", "3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/sentry", child3.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/db", child2.ID()); err != nil {
		t.Fatal(err)
	}

	child5, err := db.Set("/gograph", "5")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/same-ref-diff-name", child5.ID()); err != nil {
		t.Fatal(err)
	}

	entity := db.Get("/webapp/db/logs")
	if entity == nil {
		t.Fatal("Entity should not be nil")
	}
	if entity.ID() != "4" {
		t.Fatalf("Expected to get entity with id 4, got %s", entity.ID())
	}
}

func TestEnitiesPaths(t *testing.T) {
	db := newTestDb(t)
	_, err := db.Set("/webapp", "1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := db.Set("/db", "2")
	if err != nil {
		t.Fatal(err)
	}
	child4, err := db.Set("/logs", "4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/db/logs", child4.ID()); err != nil {
		t.Fatal(err)
	}

	child3, err := db.Set("/sentry", "3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/sentry", child3.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/db", child2.ID()); err != nil {
		t.Fatal(err)
	}

	child5, err := db.Set("/gograph", "5")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/same-ref-diff-name", child5.ID()); err != nil {
		t.Fatal(err)
	}

	out := db.List("/", -1)
	for _, p := range out.Paths() {
		t.Log(p)
	}
}

func TestDeleteRootEntity(t *testing.T) {
	db := newTestDb(t)

	if err := db.Delete("/"); err == nil {
		t.Fatal("Error should not be nil")
	}
}

func TestDeleteEntity(t *testing.T) {
	db := newTestDb(t)
	_, err := db.Set("/webapp", "1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := db.Set("/db", "2")
	if err != nil {
		t.Fatal(err)
	}
	child4, err := db.Set("/logs", "4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/db/logs", child4.ID()); err != nil {
		t.Fatal(err)
	}

	child3, err := db.Set("/sentry", "3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/sentry", child3.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/db", child2.ID()); err != nil {
		t.Fatal(err)
	}

	child5, err := db.Set("/gograph", "5")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Set("/webapp/same-ref-diff-name", child5.ID()); err != nil {
		t.Fatal(err)
	}

	if err := db.Delete("/webapp/sentry"); err != nil {
		t.Fatal(err)
	}
	entity := db.Get("/webapp/sentry")
	if entity != nil {
		t.Fatal("Entity /webapp/sentry should be nil")
	}
}

func TestCountRefs(t *testing.T) {
	db := newTestDb(t)

	db.Set("/webapp", "1")

	if db.Refs("1") != 1 {
		t.Fatal("Expect reference count to be 1")
	}

	db.Set("/db", "2")
	db.Set("/webapp/db", "2")
	if db.Refs("2") != 2 {
		t.Fatal("Expect reference count to be 2")
	}
}

func TestPurgeId(t *testing.T) {
	db := newTestDb(t)

	db.Set("/webapp", "1")

	if db.Refs("1") != 1 {
		t.Fatal("Expect reference count to be 1")
	}

	db.Set("/db", "2")
	db.Set("/webapp/db", "2")

	count, err := db.Purge("2")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("Expected 2 references to be removed")
	}
}

func TestRename(t *testing.T) {
	db := newTestDb(t)

	db.Set("/webapp", "1")

	if db.Refs("1") != 1 {
		t.Fatal("Expect reference count to be 1")
	}

	db.Set("/db", "2")
	db.Set("/webapp/db", "2")

	if db.Get("/webapp/db") == nil {
		t.Fatal("Cannot find entity at path /webapp/db")
	}

	if err := db.Rename("/webapp/db", "/webapp/newdb"); err != nil {
		t.Fatal(err)
	}
	if db.Get("/webapp/db") != nil {
		t.Fatal("Entity should not exist at /webapp/db")
	}
	if db.Get("/webapp/newdb") == nil {
		t.Fatal("Cannot find entity at path /webapp/newdb")
	}

}

func TestCreateMultipleNames(t *testing.T) {
	db := newTestDb(t)

	db.Set("/db", "1")
	if _, err := db.Set("/myapp", "1"); err != nil {
		t.Fatal(err)
	}

	db.Walk("/", func(p string, e *Entity) error {
		t.Logf("%s\n", p)
		return nil
	}, -1)
}
