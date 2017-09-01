### Notice

Do not change .pb.go files directly. You need to change the corresponding .proto files and run the following command to regenerate the .pb.go files.
```
$ make generate
```

Click [here](https://github.com/google/protobuf) for more information about protobuf.

The `api.pb.txt` file contains merged descriptors of all defined services and messages.
Definitions present here are considered frozen after the release.

At release time, the current `api.pb.txt` file will be moved into place to
freeze the API changes for the minor version. For example, when 1.0.0 is
released, `api.pb.txt` should be moved to `1.0.txt`. Notice that we leave off
the patch number, since the API will be completely locked down for a given
patch series.

We may find that by default, protobuf descriptors are too noisy to lock down
API changes. In that case, we may filter out certain fields in the descriptors,
possibly regenerating for old versions.

This process is similar to the [process used to ensure backwards compatibility
in Go](https://github.com/golang/go/tree/master/api).
