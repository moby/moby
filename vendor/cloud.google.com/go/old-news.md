_October 19, 2016_

Breaking changes to cloud.google.com/go/bigquery:

* Client.Table and Client.OpenTable have been removed.
    Replace
    ```go
    client.OpenTable("project", "dataset", "table")
    ```
    with
    ```go
    client.DatasetInProject("project", "dataset").Table("table")
    ```

* Client.CreateTable has been removed.
    Replace
    ```go
    client.CreateTable(ctx, "project", "dataset", "table")
    ```
    with
    ```go
    client.DatasetInProject("project", "dataset").Table("table").Create(ctx)
    ```

* Dataset.ListTables have been replaced with Dataset.Tables.
    Replace
    ```go
    tables, err := ds.ListTables(ctx)
    ```
    with
    ```go
    it := ds.Tables(ctx)
    for {
        table, err := it.Next()
        if err == iterator.Done {
            break
        }
        if err != nil {
            // TODO: Handle error.
        }
        // TODO: use table.
    }
    ```

* Client.Read has been replaced with Job.Read, Table.Read and Query.Read.
    Replace
    ```go
    it, err := client.Read(ctx, job)
    ```
    with
    ```go
    it, err := job.Read(ctx)
    ```
  and similarly for reading from tables or queries.

* The iterator returned from the Read methods is now named RowIterator. Its
  behavior is closer to the other iterators in these libraries. It no longer
  supports the Schema method; see the next item.
    Replace
    ```go
    for it.Next(ctx) {
        var vals ValueList
        if err := it.Get(&vals); err != nil {
            // TODO: Handle error.
        }
        // TODO: use vals.
    }
    if err := it.Err(); err != nil {
        // TODO: Handle error.
    }
    ```
    with
    ```
    for {
        var vals ValueList
        err := it.Next(&vals)
        if err == iterator.Done {
            break
        }
        if err != nil {
            // TODO: Handle error.
        }
        // TODO: use vals.
    }
    ```
    Instead of the `RecordsPerRequest(n)` option, write
    ```go
    it.PageInfo().MaxSize = n
    ```
    Instead of the `StartIndex(i)` option, write
    ```go
    it.StartIndex = i
    ```

* ValueLoader.Load now takes a Schema in addition to a slice of Values.
    Replace
    ```go
    func (vl *myValueLoader) Load(v []bigquery.Value)
    ```
    with
    ```go
    func (vl *myValueLoader) Load(v []bigquery.Value, s bigquery.Schema)
    ```


* Table.Patch is replace by Table.Update.
    Replace
    ```go
    p := table.Patch()
    p.Description("new description")
    metadata, err := p.Apply(ctx)
    ```
    with
    ```go
    metadata, err := table.Update(ctx, bigquery.TableMetadataToUpdate{
        Description: "new description",
    })
    ```

* Client.Copy is replaced by separate methods for each of its four functions.
  All options have been replaced by struct fields.

  * To load data from Google Cloud Storage into a table, use Table.LoaderFrom.

    Replace
    ```go
    client.Copy(ctx, table, gcsRef)
    ```
    with
    ```go
    table.LoaderFrom(gcsRef).Run(ctx)
    ```
    Instead of passing options to Copy, set fields on the Loader:
    ```go
    loader := table.LoaderFrom(gcsRef)
    loader.WriteDisposition = bigquery.WriteTruncate
    ```

  * To extract data from a table into Google Cloud Storage, use
    Table.ExtractorTo. Set fields on the returned Extractor instead of
    passing options.

    Replace
    ```go
    client.Copy(ctx, gcsRef, table)
    ```
    with
    ```go
    table.ExtractorTo(gcsRef).Run(ctx)
    ```

  * To copy data into a table from one or more other tables, use
    Table.CopierFrom. Set fields on the returned Copier instead of passing options.

    Replace
    ```go
    client.Copy(ctx, dstTable, srcTable)
    ```
    with
    ```go
    dst.Table.CopierFrom(srcTable).Run(ctx)
    ```

  * To start a query job, create a Query and call its Run method. Set fields
  on the query instead of passing options.

    Replace
    ```go
    client.Copy(ctx, table, query)
    ```
    with
    ```go
    query.Run(ctx)
    ```

* Table.NewUploader has been renamed to Table.Uploader. Instead of options,
  configure an Uploader by setting its fields.
    Replace
    ```go
    u := table.NewUploader(bigquery.UploadIgnoreUnknownValues())
    ```
    with
    ```go
    u := table.NewUploader(bigquery.UploadIgnoreUnknownValues())
    u.IgnoreUnknownValues = true
    ```

_October 10, 2016_

Breaking changes to cloud.google.com/go/storage:

* AdminClient replaced by methods on Client.
    Replace
    ```go
    adminClient.CreateBucket(ctx, bucketName, attrs)
    ```
    with
    ```go
    client.Bucket(bucketName).Create(ctx, projectID, attrs)
    ```

* BucketHandle.List replaced by BucketHandle.Objects.
    Replace
    ```go
    for query != nil {
        objs, err := bucket.List(d.ctx, query)
        if err != nil { ... }
        query = objs.Next
        for _, obj := range objs.Results {
            fmt.Println(obj)
        }
    }
    ```
    with
    ```go
    iter := bucket.Objects(d.ctx, query)
    for {
        obj, err := iter.Next()
        if err == iterator.Done {
            break
        }
        if err != nil { ... }
        fmt.Println(obj)
    }
    ```
    (The `iterator` package is at `google.golang.org/api/iterator`.)

    Replace `Query.Cursor` with `ObjectIterator.PageInfo().Token`.
    
    Replace `Query.MaxResults` with `ObjectIterator.PageInfo().MaxSize`.


* ObjectHandle.CopyTo replaced by ObjectHandle.CopierFrom.
    Replace
    ```go
    attrs, err := src.CopyTo(ctx, dst, nil)
    ```
    with
    ```go
    attrs, err := dst.CopierFrom(src).Run(ctx)
    ```

    Replace
    ```go
    attrs, err := src.CopyTo(ctx, dst, &storage.ObjectAttrs{ContextType: "text/html"})
    ```
    with
    ```go
    c := dst.CopierFrom(src)
    c.ContextType = "text/html"
    attrs, err := c.Run(ctx)
    ```

* ObjectHandle.ComposeFrom replaced by ObjectHandle.ComposerFrom.
    Replace
    ```go
    attrs, err := dst.ComposeFrom(ctx, []*storage.ObjectHandle{src1, src2}, nil)
    ```
    with
    ```go
    attrs, err := dst.ComposerFrom(src1, src2).Run(ctx)
    ```

* ObjectHandle.Update's ObjectAttrs argument replaced by ObjectAttrsToUpdate.
    Replace
    ```go
    attrs, err := obj.Update(ctx, &storage.ObjectAttrs{ContextType: "text/html"})
    ```
    with
    ```go
    attrs, err := obj.Update(ctx, storage.ObjectAttrsToUpdate{ContextType: "text/html"})
    ```

* ObjectHandle.WithConditions replaced by ObjectHandle.If.
    Replace
    ```go
    obj.WithConditions(storage.Generation(gen), storage.IfMetaGenerationMatch(mgen))
    ```
    with
    ```go
    obj.Generation(gen).If(storage.Conditions{MetagenerationMatch: mgen})
    ```

    Replace
    ```go
    obj.WithConditions(storage.IfGenerationMatch(0))
    ```
    with
    ```go
    obj.If(storage.Conditions{DoesNotExist: true})
    ```

* `storage.Done` replaced by `iterator.Done` (from package `google.golang.org/api/iterator`).

_October 6, 2016_

Package preview/logging deleted. Use logging instead.

_September 27, 2016_

Logging client replaced with preview version (see below).

_September 8, 2016_

* New clients for some of Google's Machine Learning APIs: Vision, Speech, and
Natural Language.

* Preview version of a new [Stackdriver Logging][cloud-logging] client in
[`cloud.google.com/go/preview/logging`](https://godoc.org/cloud.google.com/go/preview/logging).
This client uses gRPC as its transport layer, and supports log reading, sinks
and metrics. It will replace the current client at `cloud.google.com/go/logging` shortly.

