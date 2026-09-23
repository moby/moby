# Persistent record storage

Store opaque key/value records without depending on host-local paths.
Declare `storagekv.Point.Dependency()` and resolve a namespace during extension initialization:

```go
kv, err := storagekv.GetKV(resolver, "org.my.extension")
if err != nil {
    return err
}
```

Use the namespace-bound helper for record operations:

```go
err = kv.Create(ctx, "jobs/123", data)
data, err = kv.Get(ctx, "jobs/123")
err = kv.Update(ctx, "jobs/123", updatedData)
err = kv.Delete(ctx, "jobs/123")
page, err := kv.List(ctx, storagekv.ListOptions{Prefix: "jobs/"})
```

- The same namespace shares records; it is not an authentication boundary.
- Keys are logical strings, not paths; consumers own value encoding and schema versions.
- `Create` requires an absent key; `Update` requires an existing key; `Delete` tolerates absence.
- Writes are durable and atomic per record, with no multi-record transactions or concurrent-update protection.
- Listing is paginated, not a snapshot; continue with `NextCursor` using the same prefix.
- The helper needs no `Close`; records survive provider restarts and extension removal.

Executable consumers register the generated `protogen.ClientPoint` with their SDK client-point registry.
See [storage.go](storage.go) for the full contract and size limits.
