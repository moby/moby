// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	stderrors "errors"
	"net/http"
	"strings"

	"github.com/go-openapi/errors"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/server-middleware/mediatype"
)

type validation struct {
	context *Context
	result  []error
	request *http.Request
	route   *MatchedRoute
	bound   map[string]any
}

// validateContentType maps [mediatype.MatchFirst] to the runtime's
// validation errors:
//
//   - actual fails to parse        → HTTP 400 ([errors.NewParseError]).
//   - actual is well-formed but
//     no allowed entry accepts it  → HTTP 415 ([errors.InvalidContentType]).
//
// In the standard runtime flow, malformed Content-Type headers are
// already caught upstream by [runtime.ContentType] (which itself returns
// a 400 [errors.ParseError]). This function therefore only sees the
// malformed case when invoked directly by callers that have bypassed
// that step.
func validateContentType(allowed []string, actual string, opts ...mediatype.MatchOption) error {
	if len(allowed) == 0 {
		return nil
	}
	_, ok, err := mediatype.MatchFirst(allowed, actual, opts...)
	if ok {
		return nil
	}
	if err != nil {
		return errors.NewParseError(runtime.HeaderContentType, "header", actual, err)
	}
	return errors.InvalidContentType(actual, allowed)
}

func validateRequest(ctx *Context, request *http.Request, route *MatchedRoute) *validation {
	validate := &validation{
		context: ctx,
		request: request,
		route:   route,
		bound:   make(map[string]any),
	}
	validate.debugLogf("validating request %s %s", request.Method, request.URL.EscapedPath())

	validate.contentType()
	if len(validate.result) == 0 {
		validate.responseFormat()
	}
	if len(validate.result) == 0 {
		validate.parameters()
	}

	return validate
}

func (v *validation) debugLogf(format string, args ...any) {
	v.context.debugLogf(format, args...)
}

func (v *validation) parameters() {
	v.debugLogf("validating request parameters for %s %s", v.request.Method, v.request.URL.EscapedPath())
	result := v.route.Binder.bind(v.request, v.route.Params, v.route.Consumer, v.bound)
	if result == nil {
		return
	}

	for _, e := range result.Errors {
		var validationErr *errors.Validation
		if stderrors.As(e, &validationErr) {
			v.result = append(v.result, validationErr)
		}
	}
}

func (v *validation) contentType() {
	if len(v.result) > 0 || !runtime.HasBody(v.request) {
		return
	}

	v.debugLogf("validating body content type for %s %s", v.request.Method, v.request.URL.EscapedPath())
	ct, _, req, err := v.context.ContentType(v.request)
	if err != nil {
		v.result = append(v.result, err)
	} else {
		v.request = req
	}

	if len(v.result) == 0 {
		v.debugLogf("validating content type for %q against [%s]", ct, strings.Join(v.route.Consumes, ", "))
		if err := validateContentType(v.route.Consumes, ct, v.context.matchOpts()...); err != nil {
			v.result = append(v.result, err)
		}
	}

	if ct == "" || v.route.Consumer != nil {
		return
	}

	cons, ok := mediatype.Lookup(v.route.Consumers, ct, v.context.matchOpts()...)
	if !ok {
		v.result = append(v.result, errors.New(http.StatusInternalServerError, "no consumer registered for %s", ct))
	} else {
		v.route.Consumer = cons
	}
}

func (v *validation) responseFormat() {
	// if the route provides values for Produces and no format could be identified then return an error.
	// if the route does not specify values for Produces then treat request as valid since the API designer
	// choose not to specify the format for responses.
	if str, rCtx := v.context.ResponseFormat(v.request, v.route.Produces); str == "" && len(v.route.Produces) > 0 {
		v.request = rCtx
		v.result = append(v.result, errors.InvalidResponseFormat(v.request.Header.Get(runtime.HeaderAccept), v.route.Produces))
	}
}
