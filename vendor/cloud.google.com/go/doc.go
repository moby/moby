// Copyright 2014 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package cloud is the root of the packages used to access Google Cloud
Services. See https://godoc.org/cloud.google.com/go for a full list
of sub-packages.

# Client Options

All clients in sub-packages are configurable via client options. These options are
described here: https://godoc.org/google.golang.org/api/option.

## Endpoint Override

Endpoint configuration is used to specify the URL to which requests are
sent. It is used for services that support or require regional endpoints, as well
as for other use cases such as [testing against fake
servers](https://github.com/googleapis/google-cloud-go/blob/main/testing.md#testing-grpc-services-using-fakes).

For example, the Vertex AI service recommends that you configure the endpoint to the
location with the features you want that is closest to your physical location or the
location of your users. There is no global endpoint for Vertex AI. See
[Vertex AI - Locations](https://cloud.google.com/vertex-ai/docs/general/locations)
for more details. The following example demonstrates configuring a Vertex AI client
with a regional endpoint:

	ctx := context.Background()
	endpoint := "us-central1-aiplatform.googleapis.com:443"
	client, err := aiplatform.NewDatasetClient(ctx, option.WithEndpoint(endpoint))

# Authentication and Authorization

All the clients in sub-packages support authentication via Google Application Default
Credentials (see https://cloud.google.com/docs/authentication/production), or
by providing a JSON key file for a Service Account. See examples below.

Google Application Default Credentials (ADC) is the recommended way to authorize
and authenticate clients. For information on how to create and obtain
Application Default Credentials, see
https://cloud.google.com/docs/authentication/production. Here is an example
of a client using ADC to authenticate:

	client, err := secretmanager.NewClient(context.Background())
	if err != nil {
		// TODO: handle error.
	}
	_ = client // Use the client.

You can use a file with credentials to authenticate and authorize, such as a JSON
key file associated with a Google service account. Service Account keys can be
created and downloaded from
https://console.cloud.google.com/iam-admin/serviceaccounts. This example uses
the Secret Manger client, but the same steps apply to the other client libraries
underneath this package. Example:

	client, err := secretmanager.NewClient(context.Background(),
		option.WithCredentialsFile("/path/to/service-account-key.json"))
	if err != nil {
		// TODO: handle error.
	}
	_ = client // Use the client.

In some cases (for instance, you don't want to store secrets on disk), you can
create credentials from in-memory JSON and use the WithCredentials option.
The google package in this example is at golang.org/x/oauth2/google.
This example uses the Secret Manager client, but the same steps apply to
the other client libraries underneath this package. Note that scopes can be
found at https://developers.google.com/identity/protocols/oauth2/scopes, and
are also provided in all auto-generated libraries: for example,
cloud.google.com/go/secretmanager/apiv1 provides DefaultAuthScopes. Example:

	ctx := context.Background()
	creds, err := google.CredentialsFromJSON(ctx, []byte("JSON creds"), secretmanager.DefaultAuthScopes()...)
	if err != nil {
		// TODO: handle error.
	}
	client, err := secretmanager.NewClient(ctx, option.WithCredentials(creds))
	if err != nil {
		// TODO: handle error.
	}
	_ = client // Use the client.

# Timeouts and Cancellation

By default, non-streaming methods, like Create or Get, will have a default deadline applied to the
context provided at call time, unless a context deadline is already set. Streaming
methods have no default deadline and will run indefinitely. To set timeouts or
arrange for cancellation, use contexts. Transient
errors will be retried when correctness allows.

Here is an example of how to set a timeout for an RPC, use context.WithTimeout:

	ctx := context.Background()
	// Do not set a timeout on the context passed to NewClient: dialing happens
	// asynchronously, and the context is used to refresh credentials in the
	// background.
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		// TODO: handle error.
	}
	// Time out if it takes more than 10 seconds to create a dataset.
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel() // Always call cancel.

	req := &secretmanagerpb.DeleteSecretRequest{Name: "projects/project-id/secrets/name"}
	if err := client.DeleteSecret(tctx, req); err != nil {
		// TODO: handle error.
	}

Here is an example of how to arrange for an RPC to be canceled, use context.WithCancel:

	ctx := context.Background()
	// Do not cancel the context passed to NewClient: dialing happens asynchronously,
	// and the context is used to refresh credentials in the background.
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		// TODO: handle error.
	}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel() // Always call cancel.

	// TODO: Make the cancel function available to whatever might want to cancel the
	// call--perhaps a GUI button.
	req := &secretmanagerpb.DeleteSecretRequest{Name: "projects/proj/secrets/name"}
	if err := client.DeleteSecret(cctx, req); err != nil {
		// TODO: handle error.
	}

To opt out of default deadlines, set the temporary environment variable
GOOGLE_API_GO_EXPERIMENTAL_DISABLE_DEFAULT_DEADLINE to "true" prior to client
creation. This affects all Google Cloud Go client libraries. This opt-out
mechanism will be removed in a future release. File an issue at
https://github.com/googleapis/google-cloud-go if the default deadlines
cannot work for you.

Do not attempt to control the initial connection (dialing) of a service by setting a
timeout on the context passed to NewClient. Dialing is non-blocking, so timeouts
would be ineffective and would only interfere with credential refreshing, which uses
the same context.

# Connection Pooling

Connection pooling differs in clients based on their transport. Cloud
clients either rely on HTTP or gRPC transports to communicate
with Google Cloud.

Cloud clients that use HTTP (bigquery, compute, storage, and translate) rely on the
underlying HTTP transport to cache connections for later re-use. These are cached to
the default http.MaxIdleConns and http.MaxIdleConnsPerHost settings in
http.DefaultTransport.

For gRPC clients (all others in this repo), connection pooling is configurable. Users
of cloud client libraries may specify option.WithGRPCConnectionPool(n) as a client
option to NewClient calls. This configures the underlying gRPC connections to be
pooled and addressed in a round robin fashion.

# Using the Libraries with Docker

Minimal docker images like Alpine lack CA certificates. This causes RPCs to appear to
hang, because gRPC retries indefinitely. See https://github.com/googleapis/google-cloud-go/issues/928
for more information.

# Debugging

To see gRPC logs, set the environment variable GRPC_GO_LOG_SEVERITY_LEVEL. See
https://godoc.org/google.golang.org/grpc/grpclog for more information.

For HTTP logging, set the GODEBUG environment variable to "http2debug=1" or "http2debug=2".

# Inspecting errors

Most of the errors returned by the generated clients are wrapped in an
[github.com/googleapis/gax-go/v2/apierror.APIError] and can be further unwrapped
into a [google.golang.org/grpc/status.Status] or
[google.golang.org/api/googleapi.Error] depending
on the transport used to make the call (gRPC or REST). Converting your errors to
these types can be a useful way to get more information about what went wrong
while debugging.

[github.com/googleapis/gax-go/v2/apierror.APIError] gives access to specific
details in the error. The transport-specific errors can still be unwrapped using
the [github.com/googleapis/gax-go/v2/apierror.APIError].

	if err != nil {
	   var ae *apierror.APIError
	   if errors.As(err, &ae) {
	      log.Println(ae.Reason())
	      log.Println(ae.Details().Help.GetLinks())
	   }
	}

If the gRPC transport was used, the [google.golang.org/grpc/status.Status] can
still be parsed using the [google.golang.org/grpc/status.FromError] function.

	if err != nil {
	   if s, ok := status.FromError(err); ok {
	      log.Println(s.Message())
	      for _, d := range s.Proto().Details {
	         log.Println(d)
	      }
	   }
	}

If the REST transport was used, the [google.golang.org/api/googleapi.Error] can
be parsed in a similar way, allowing access to details such as the HTTP response
code.

	if err != nil {
	   var gerr *googleapi.Error
	   if errors.As(err, &gerr) {
	      log.Println(gerr.Message)
	   }
	}

# Client Stability

Clients in this repository are considered alpha or beta unless otherwise
marked as stable in the README.md. Semver is not used to communicate stability
of clients.

Alpha and beta clients may change or go away without notice.

Clients marked stable will maintain compatibility with future versions for as
long as we can reasonably sustain. Incompatible changes might be made in some
situations, including:

- Security bugs may prompt backwards-incompatible changes.

- Situations in which components are no longer feasible to maintain without
making breaking changes, including removal.

- Parts of the client surface may be outright unstable and subject to change.
These parts of the surface will be labeled with the note, "It is EXPERIMENTAL
and subject to change or removal without notice."
*/
package cloud // import "cloud.google.com/go"
