# Long Running Operations API

[![Go Reference](https://pkg.go.dev/badge/cloud.google.com/go/longrunning.svg)](https://pkg.go.dev/cloud.google.com/go/longrunning)

Go Client Library for Long Running Operations API.

## Install

```bash
go get cloud.google.com/go/longrunning
```

## Stability

The stability of this module is indicated by SemVer.

However, a `v1+` module may have breaking changes in two scenarios:

* Packages with `alpha` or `beta` in the import path
* The GoDoc has an explicit stability disclaimer (for example, for an experimental feature).

### Which package to use?

Generated client library surfaces can be found in packages whose import path
ends in `.../apivXXX`. The `XXX` could be something like `1` or `2` in the case
of a stable service backend or may be like `1beta2` or `2beta` in the case of a
more experimental service backend. Because of this fact, a given module can have
multiple clients for different service backends. In these cases it is generally
recommended to use clients with stable service backends, with import suffixes like
`apiv1`, unless you need to use features that are only present in a beta backend
or there is not yet a stable backend available.

## Google Cloud Samples

To browse ready to use code samples check [Google Cloud Samples](https://cloud.google.com/docs/samples?l=go).

## Go Version Support

See the [Go Versions Supported](https://github.com/googleapis/google-cloud-go#go-versions-supported)
section in the root directory's README.

## Authorization

See the [Authorization](https://github.com/googleapis/google-cloud-go#authorization)
section in the root directory's README.

## Contributing

Contributions are welcome. Please, see the [CONTRIBUTING](https://github.com/GoogleCloudPlatform/google-cloud-go/blob/main/CONTRIBUTING.md)
document for details.

Please note that this project is released with a Contributor Code of Conduct.
By participating in this project you agree to abide by its terms. See
[Contributor Code of Conduct](https://github.com/GoogleCloudPlatform/google-cloud-go/blob/main/CONTRIBUTING.md#contributor-code-of-conduct)
for more information.
