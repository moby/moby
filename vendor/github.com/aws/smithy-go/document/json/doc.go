// Package json provides a document Encoder and Decoder implementation that is used to implement Smithy document types
// for JSON based protocols. The Encoder and Decoder implement the document.Marshaler and document.Unmarshaler
// interfaces respectively.
//
// This package handles protocol specific implementation details about documents, and can not be used to construct
// a document type for a service client. To construct a document type see each service clients respective document
// package and NewLazyDocument function.
package json
