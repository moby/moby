package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aws/smithy-go"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type buildEndpoint struct {
	Endpoint string
}

func (b *buildEndpoint) ID() string {
	return "BuildEndpoint"
}

func (b *buildEndpoint) HandleBuild(ctx context.Context, in smithymiddleware.BuildInput, next smithymiddleware.BuildHandler) (
	out smithymiddleware.BuildOutput, metadata smithymiddleware.Metadata, err error,
) {
	request, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, fmt.Errorf("unknown transport, %T", in.Request)
	}

	if len(b.Endpoint) == 0 {
		return out, metadata, fmt.Errorf("endpoint not provided")
	}

	parsed, err := url.Parse(b.Endpoint)
	if err != nil {
		return out, metadata, fmt.Errorf("failed to parse endpoint, %w", err)
	}

	request.URL = parsed

	return next.HandleBuild(ctx, in)
}

type serializeOpGetCredential struct{}

func (s *serializeOpGetCredential) ID() string {
	return "OperationSerializer"
}

func (s *serializeOpGetCredential) HandleSerialize(ctx context.Context, in smithymiddleware.SerializeInput, next smithymiddleware.SerializeHandler) (
	out smithymiddleware.SerializeOutput, metadata smithymiddleware.Metadata, err error,
) {
	request, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, fmt.Errorf("unknown transport type, %T", in.Request)
	}

	params, ok := in.Parameters.(*GetCredentialsInput)
	if !ok {
		return out, metadata, fmt.Errorf("unknown input parameters, %T", in.Parameters)
	}

	const acceptHeader = "Accept"
	request.Header[acceptHeader] = append(request.Header[acceptHeader][:0], "application/json")

	if len(params.AuthorizationToken) > 0 {
		const authHeader = "Authorization"
		request.Header[authHeader] = append(request.Header[authHeader][:0], params.AuthorizationToken)
	}

	return next.HandleSerialize(ctx, in)
}

type deserializeOpGetCredential struct{}

func (d *deserializeOpGetCredential) ID() string {
	return "OperationDeserializer"
}

func (d *deserializeOpGetCredential) HandleDeserialize(ctx context.Context, in smithymiddleware.DeserializeInput, next smithymiddleware.DeserializeHandler) (
	out smithymiddleware.DeserializeOutput, metadata smithymiddleware.Metadata, err error,
) {
	out, metadata, err = next.HandleDeserialize(ctx, in)
	if err != nil {
		return out, metadata, err
	}

	response, ok := out.RawResponse.(*smithyhttp.Response)
	if !ok {
		return out, metadata, &smithy.DeserializationError{Err: fmt.Errorf("unknown transport type %T", out.RawResponse)}
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return out, metadata, deserializeError(response)
	}

	var shape *GetCredentialsOutput
	if err = json.NewDecoder(response.Body).Decode(&shape); err != nil {
		return out, metadata, &smithy.DeserializationError{Err: fmt.Errorf("failed to deserialize json response, %w", err)}
	}

	out.Result = shape
	return out, metadata, err
}

func deserializeError(response *smithyhttp.Response) error {
	var errShape *EndpointError
	err := json.NewDecoder(response.Body).Decode(&errShape)
	if err != nil {
		return &smithy.DeserializationError{Err: fmt.Errorf("failed to decode error message, %w", err)}
	}

	if response.StatusCode >= 500 {
		errShape.Fault = smithy.FaultServer
	} else {
		errShape.Fault = smithy.FaultClient
	}

	return errShape
}
