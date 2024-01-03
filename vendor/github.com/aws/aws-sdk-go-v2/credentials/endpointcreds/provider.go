// Package endpointcreds provides support for retrieving credentials from an
// arbitrary HTTP endpoint.
//
// The credentials endpoint Provider can receive both static and refreshable
// credentials that will expire. Credentials are static when an "Expiration"
// value is not provided in the endpoint's response.
//
// Static credentials will never expire once they have been retrieved. The format
// of the static credentials response:
//
//	{
//	    "AccessKeyId" : "MUA...",
//	    "SecretAccessKey" : "/7PC5om....",
//	}
//
// Refreshable credentials will expire within the "ExpiryWindow" of the Expiration
// value in the response. The format of the refreshable credentials response:
//
//	{
//	    "AccessKeyId" : "MUA...",
//	    "SecretAccessKey" : "/7PC5om....",
//	    "Token" : "AQoDY....=",
//	    "Expiration" : "2016-02-25T06:03:31Z"
//	}
//
// Errors should be returned in the following format and only returned with 400
// or 500 HTTP status codes.
//
//	{
//	    "code": "ErrorCode",
//	    "message": "Helpful error message."
//	}
package endpointcreds

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/endpointcreds/internal/client"
	"github.com/aws/smithy-go/middleware"
)

// ProviderName is the name of the credentials provider.
const ProviderName = `CredentialsEndpointProvider`

type getCredentialsAPIClient interface {
	GetCredentials(context.Context, *client.GetCredentialsInput, ...func(*client.Options)) (*client.GetCredentialsOutput, error)
}

// Provider satisfies the aws.CredentialsProvider interface, and is a client to
// retrieve credentials from an arbitrary endpoint.
type Provider struct {
	// The AWS Client to make HTTP requests to the endpoint with. The endpoint
	// the request will be made to is provided by the aws.Config's
	// EndpointResolver.
	client getCredentialsAPIClient

	options Options
}

// HTTPClient is a client for sending HTTP requests
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Options is structure of configurable options for Provider
type Options struct {
	// Endpoint to retrieve credentials from. Required
	Endpoint string

	// HTTPClient to handle sending HTTP requests to the target endpoint.
	HTTPClient HTTPClient

	// Set of options to modify how the credentials operation is invoked.
	APIOptions []func(*middleware.Stack) error

	// The Retryer to be used for determining whether a failed requested should be retried
	Retryer aws.Retryer

	// Optional authorization token value if set will be used as the value of
	// the Authorization header of the endpoint credential request.
	AuthorizationToken string
}

// New returns a credentials Provider for retrieving AWS credentials
// from arbitrary endpoint.
func New(endpoint string, optFns ...func(*Options)) *Provider {
	o := Options{
		Endpoint: endpoint,
	}

	for _, fn := range optFns {
		fn(&o)
	}

	p := &Provider{
		client: client.New(client.Options{
			HTTPClient: o.HTTPClient,
			Endpoint:   o.Endpoint,
			APIOptions: o.APIOptions,
			Retryer:    o.Retryer,
		}),
		options: o,
	}

	return p
}

// Retrieve will attempt to request the credentials from the endpoint the Provider
// was configured for. And error will be returned if the retrieval fails.
func (p *Provider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	resp, err := p.getCredentials(ctx)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to load credentials, %w", err)
	}

	creds := aws.Credentials{
		AccessKeyID:     resp.AccessKeyID,
		SecretAccessKey: resp.SecretAccessKey,
		SessionToken:    resp.Token,
		Source:          ProviderName,
	}

	if resp.Expiration != nil {
		creds.CanExpire = true
		creds.Expires = *resp.Expiration
	}

	return creds, nil
}

func (p *Provider) getCredentials(ctx context.Context) (*client.GetCredentialsOutput, error) {
	return p.client.GetCredentials(ctx, &client.GetCredentialsInput{AuthorizationToken: p.options.AuthorizationToken})
}
