# tar2go

tar2go implements are go [fs.FS](https://pkg.go.dev/io/fs#FS) for tar files.

Tars are not indexed so by themselves don't really have support for random access.
When a request to open/stat a file is made tar2go will scan through the tar, indexing each entry along the way, until the file is found in the tar.
A tar file is only ever scanned 1 time and scanning is done lazily (as needed to index the requested entry).

tar2go does not support modifying a tar file, however there is support for modifying the in-memory representation of the tar which will show up in the `fs.FS`.
You can also write a new tar file with requested modifications.

### Usage

```go
  f, _ := os.Open(p)
  defer f.Close()
  
  // Entrypoint into this library
  idx := NewIndex(f)
  
  // Get the `fs.FS` implementation
  goFS := idx.FS()
  // Do stuff with your fs
  // ...
  
  
  // Add or replace a file in the index
  _ := idx.Replace("foo", strings.NewReader("random stuff")
  data, _ := fs.ReadFile(goFS, "foo")
  if string(data) != "random stuff") {
    panic("unexpected data")
  }
  
  // Delete a file in the index
  _ := idx.Replace("foo", nil)
  if _, err := fs.ReadFile(goFS, "foo"); !errors.Is(err, fs.ErrNotExist) {
    panic(err)
  }
  
  // Create a new tar with updated content
  // First we need to create an `io.Writer`, which is where the updated tar stream will be written to.
  f, _ := os.CreateTemp("", "updated")
  idx.Update(f, func(name string, rdr ReaderAtSized) (ReaderAtSized, bool, error) {
    // Update calls this function for every file in the tar
    // The returned `ReaderAtSized` is used instead of the content passed in (rdr).
    // To make no changes just return the same rdr back.
    // Return true for the bool value if the content is changed.
  })
```
