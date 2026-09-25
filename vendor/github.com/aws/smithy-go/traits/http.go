package traits

import smithy "github.com/aws/smithy-go"

// HTTPHeader represents smithy.api#httpHeader.
type HTTPHeader struct {
	Name string
}

// TraitID identifies the trait.
func (*HTTPHeader) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "httpHeader"} }

// HTTPLabel represents smithy.api#httpLabel.
type HTTPLabel struct{}

// TraitID identifies the trait.
func (*HTTPLabel) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "httpLabel"} }

// HTTPPayload represents smithy.api#httpPayload.
type HTTPPayload struct{}

// TraitID identifies the trait.
func (*HTTPPayload) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "httpPayload"} }

// HTTPPrefixHeaders represents smithy.api#httpPrefixHeaders.
type HTTPPrefixHeaders struct {
	Prefix string
}

// TraitID identifies the trait.
func (*HTTPPrefixHeaders) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "httpPrefixHeaders"} }

// HTTPQuery represents smithy.api#httpQuery.
type HTTPQuery struct {
	Name string
}

// TraitID identifies the trait.
func (*HTTPQuery) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "httpQuery"} }

// HTTPQueryParams represents smithy.api#httpQueryParams.
type HTTPQueryParams struct{}

// TraitID identifies the trait.
func (*HTTPQueryParams) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "httpQueryParams"} }

// HTTPResponseCode represents smithy.api#httpResponseCode.
type HTTPResponseCode struct{}

// TraitID identifies the trait.
func (*HTTPResponseCode) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "httpResponseCode"} }

// HTTP represents smithy.api#http.
type HTTP struct {
	Method string
	URI    string
	Code   int
}

// TraitID identifies the trait.
func (*HTTP) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "http"} }

// HTTPError represents smithy.api#httpError.
type HTTPError struct {
	Code int
}

// TraitID identifies the trait.
func (*HTTPError) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "httpError"} }
