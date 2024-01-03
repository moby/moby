# go-memdb [![CircleCI](https://circleci.com/gh/hashicorp/go-memdb/tree/master.svg?style=svg)](https://circleci.com/gh/hashicorp/go-memdb/tree/master)

Provides the `memdb` package that implements a simple in-memory database
built on immutable radix trees. The database provides Atomicity, Consistency
and Isolation from ACID. Being that it is in-memory, it does not provide durability.
The database is instantiated with a schema that specifies the tables and indices
that exist and allows transactions to be executed.

The database provides the following:

* Multi-Version Concurrency Control (MVCC) - By leveraging immutable radix trees
  the database is able to support any number of concurrent readers without locking,
  and allows a writer to make progress.

* Transaction Support - The database allows for rich transactions, in which multiple
  objects are inserted, updated or deleted. The transactions can span multiple tables,
  and are applied atomically. The database provides atomicity and isolation in ACID
  terminology, such that until commit the updates are not visible.

* Rich Indexing - Tables can support any number of indexes, which can be simple like
  a single field index, or more advanced compound field indexes. Certain types like
  UUID can be efficiently compressed from strings into byte indexes for reduced
  storage requirements.

* Watches - Callers can populate a watch set as part of a query, which can be used to
  detect when a modification has been made to the database which affects the query
  results. This lets callers easily watch for changes in the database in a very general
  way.

For the underlying immutable radix trees, see [go-immutable-radix](https://github.com/hashicorp/go-immutable-radix).

Documentation
=============

The full documentation is available on [Godoc](https://pkg.go.dev/github.com/hashicorp/go-memdb).

Example
=======

Below is a [simple example](https://play.golang.org/p/gCGE9FA4og1) of usage

```go
// Create a sample struct
type Person struct {
	Email string
	Name  string
	Age   int
}

// Create the DB schema
schema := &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		"person": &memdb.TableSchema{
			Name: "person",
			Indexes: map[string]*memdb.IndexSchema{
				"id": &memdb.IndexSchema{
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: "Email"},
				},
				"age": &memdb.IndexSchema{
					Name:    "age",
					Unique:  false,
					Indexer: &memdb.IntFieldIndex{Field: "Age"},
				},
			},
		},
	},
}

// Create a new data base
db, err := memdb.NewMemDB(schema)
if err != nil {
	panic(err)
}

// Create a write transaction
txn := db.Txn(true)

// Insert some people
people := []*Person{
	&Person{"joe@aol.com", "Joe", 30},
	&Person{"lucy@aol.com", "Lucy", 35},
	&Person{"tariq@aol.com", "Tariq", 21},
	&Person{"dorothy@aol.com", "Dorothy", 53},
}
for _, p := range people {
	if err := txn.Insert("person", p); err != nil {
		panic(err)
	}
}

// Commit the transaction
txn.Commit()

// Create read-only transaction
txn = db.Txn(false)
defer txn.Abort()

// Lookup by email
raw, err := txn.First("person", "id", "joe@aol.com")
if err != nil {
	panic(err)
}

// Say hi!
fmt.Printf("Hello %s!\n", raw.(*Person).Name)

// List all the people
it, err := txn.Get("person", "id")
if err != nil {
	panic(err)
}

fmt.Println("All the people:")
for obj := it.Next(); obj != nil; obj = it.Next() {
	p := obj.(*Person)
	fmt.Printf("  %s\n", p.Name)
}

// Range scan over people with ages between 25 and 35 inclusive
it, err = txn.LowerBound("person", "age", 25)
if err != nil {
	panic(err)
}

fmt.Println("People aged 25 - 35:")
for obj := it.Next(); obj != nil; obj = it.Next() {
	p := obj.(*Person)
	if p.Age > 35 {
		break
	}
	fmt.Printf("  %s is aged %d\n", p.Name, p.Age)
}
// Output:
// Hello Joe!
// All the people:
//   Dorothy
//   Joe
//   Lucy
//   Tariq
// People aged 25 - 35:
//   Joe is aged 30
//   Lucy is aged 35
```

