# Google Cloud Client Libraries for Go

[![Go Reference](https://pkg.go.dev/badge/cloud.google.com/go.svg)](https://pkg.go.dev/cloud.google.com/go)

Go packages for [Google Cloud Platform](https://cloud.google.com) services.

``` go
import "cloud.google.com/go"
```

To install the packages on your system, *do not clone the repo*. Instead:

1. Change to your project directory:

   ```bash
   cd /my/cloud/project
   ```
1. Get the package you want to use. Some products have their own module, so it's
   best to `go get` the package(s) you want to use:

   ```
   $ go get cloud.google.com/go/firestore # Replace with the package you want to use.
   ```

**NOTE:** Some of these packages are under development, and may occasionally
make backwards-incompatible changes.

## Supported APIs

For an updated list of all of our released APIs please see our
[reference docs](https://cloud.google.com/go/docs/reference).

## [Go Versions Supported](#supported-versions)

Our libraries are compatible with at least the three most recent, major Go
releases. They are currently compatible with:

- Go 1.20
- Go 1.19
- Go 1.18
- Go 1.17

## Authorization

By default, each API will use [Google Application Default Credentials](https://developers.google.com/identity/protocols/application-default-credentials)
for authorization credentials used in calling the API endpoints. This will allow your
application to run in many environments without requiring explicit configuration.

[snip]:# (auth)
```go
client, err := storage.NewClient(ctx)
```

To authorize using a
[JSON key file](https://cloud.google.com/iam/docs/managing-service-account-keys),
pass
[`option.WithCredentialsFile`](https://pkg.go.dev/google.golang.org/api/option#WithCredentialsFile)
to the `NewClient` function of the desired package. For example:

[snip]:# (auth-JSON)
```go
client, err := storage.NewClient(ctx, option.WithCredentialsFile("path/to/keyfile.json"))
```

You can exert more control over authorization by using the
[`golang.org/x/oauth2`](https://pkg.go.dev/golang.org/x/oauth2) package to
create an `oauth2.TokenSource`. Then pass
[`option.WithTokenSource`](https://pkg.go.dev/google.golang.org/api/option#WithTokenSource)
to the `NewClient` function:
[snip]:# (auth-ts)
```go
tokenSource := ...
client, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
```

## Contributing

Contributions are welcome. Please, see the
[CONTRIBUTING](https://github.com/GoogleCloudPlatform/google-cloud-go/blob/main/CONTRIBUTING.md)
document for details.

Please note that this project is released with a Contributor Code of Conduct.
By participating in this project you agree to abide by its terms.
See [Contributor Code of Conduct](https://github.com/GoogleCloudPlatform/google-cloud-go/blob/main/CONTRIBUTING.md#contributor-code-of-conduct)
for more information.

[cloud-asset]: https://cloud.google.com/security-command-center/docs/how-to-asset-inventory
[cloud-automl]: https://cloud.google.com/automl
[cloud-build]: https://cloud.google.com/cloud-build/
[cloud-bigquery]: https://cloud.google.com/bigquery/
[cloud-bigtable]: https://cloud.google.com/bigtable/
[cloud-compute]: https://cloud.google.com/compute
[cloud-container]: https://cloud.google.com/containers/
[cloud-containeranalysis]: https://cloud.google.com/container-registry/docs/container-analysis
[cloud-dataproc]: https://cloud.google.com/dataproc/
[cloud-datastore]: https://cloud.google.com/datastore/
[cloud-dialogflow]: https://cloud.google.com/dialogflow-enterprise/
[cloud-debugger]: https://cloud.google.com/debugger/
[cloud-dlp]: https://cloud.google.com/dlp/
[cloud-errors]: https://cloud.google.com/error-reporting/
[cloud-firestore]: https://cloud.google.com/firestore/
[cloud-iam]: https://cloud.google.com/iam/
[cloud-iot]: https://cloud.google.com/iot-core/
[cloud-irm]: https://cloud.google.com/incident-response/docs/concepts
[cloud-kms]: https://cloud.google.com/kms/
[cloud-pubsub]: https://cloud.google.com/pubsub/
[cloud-pubsublite]: https://cloud.google.com/pubsub/lite
[cloud-storage]: https://cloud.google.com/storage/
[cloud-language]: https://cloud.google.com/natural-language
[cloud-logging]: https://cloud.google.com/logging/
[cloud-natural-language]: https://cloud.google.com/natural-language/
[cloud-memorystore]: https://cloud.google.com/memorystore/
[cloud-monitoring]: https://cloud.google.com/monitoring/
[cloud-oslogin]: https://cloud.google.com/compute/docs/oslogin/rest
[cloud-phishingprotection]: https://cloud.google.com/phishing-protection/
[cloud-securitycenter]: https://cloud.google.com/security-command-center/
[cloud-scheduler]: https://cloud.google.com/scheduler
[cloud-spanner]: https://cloud.google.com/spanner/
[cloud-speech]: https://cloud.google.com/speech
[cloud-talent]: https://cloud.google.com/solutions/talent-solution/
[cloud-tasks]: https://cloud.google.com/tasks/
[cloud-texttospeech]: https://cloud.google.com/texttospeech/
[cloud-talent]: https://cloud.google.com/solutions/talent-solution/
[cloud-trace]: https://cloud.google.com/trace/
[cloud-translate]: https://cloud.google.com/translate
[cloud-recaptcha]: https://cloud.google.com/recaptcha-enterprise/
[cloud-recommender]: https://cloud.google.com/recommendations/
[cloud-video]: https://cloud.google.com/video-intelligence/
[cloud-vision]: https://cloud.google.com/vision
[cloud-webrisk]: https://cloud.google.com/web-risk/

## Links

- [Go on Google Cloud](https://cloud.google.com/go/home)
- [Getting started with Go on Google Cloud](https://cloud.google.com/go/getting-started)
- [App Engine Quickstart](https://cloud.google.com/appengine/docs/standard/go/quickstart)
- [Cloud Functions Quickstart](https://cloud.google.com/functions/docs/quickstart-go)
- [Cloud Run Quickstart](https://cloud.google.com/run/docs/quickstarts/build-and-deploy#go)
