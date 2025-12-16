// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"reflect"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/logger"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
)

// UntypedRequestBinder binds and validates the data from a http request
type UntypedRequestBinder struct {
	Spec         *spec.Swagger
	Parameters   map[string]spec.Parameter
	Formats      strfmt.Registry
	paramBinders map[string]*untypedParamBinder
	debugLogf    func(string, ...any) // a logging function to debug context and all components using it
}

// NewUntypedRequestBinder creates a new binder for reading a request.
func NewUntypedRequestBinder(parameters map[string]spec.Parameter, spec *spec.Swagger, formats strfmt.Registry) *UntypedRequestBinder {
	binders := make(map[string]*untypedParamBinder)
	for fieldName, param := range parameters {
		binders[fieldName] = newUntypedParamBinder(param, spec, formats)
	}
	return &UntypedRequestBinder{
		Parameters:   parameters,
		paramBinders: binders,
		Spec:         spec,
		Formats:      formats,
		debugLogf:    debugLogfFunc(nil),
	}
}

// Bind perform the databinding and validation
func (o *UntypedRequestBinder) Bind(request *http.Request, routeParams RouteParams, consumer runtime.Consumer, data any) error {
	val := reflect.Indirect(reflect.ValueOf(data))
	isMap := val.Kind() == reflect.Map
	var result []error
	o.debugLogf("binding %d parameters for %s %s", len(o.Parameters), request.Method, request.URL.EscapedPath())
	for fieldName, param := range o.Parameters {
		binder := o.paramBinders[fieldName]
		o.debugLogf("binding parameter %s for %s %s", fieldName, request.Method, request.URL.EscapedPath())
		var target reflect.Value
		if !isMap {
			binder.Name = fieldName
			target = val.FieldByName(fieldName)
		}

		if isMap {
			tpe := binder.Type()
			if tpe == nil {
				if param.Schema.Type.Contains(typeArray) {
					tpe = reflect.TypeFor[[]any]()
				} else {
					tpe = reflect.TypeFor[map[string]any]()
				}
			}
			target = reflect.Indirect(reflect.New(tpe))
		}

		if !target.IsValid() {
			result = append(result, errors.New(http.StatusInternalServerError, "parameter name %q is an unknown field", binder.Name))
			continue
		}

		if err := binder.Bind(request, routeParams, consumer, target); err != nil {
			result = append(result, err)
			continue
		}

		if binder.validator != nil {
			rr := binder.validator.Validate(target.Interface())
			if rr != nil && rr.HasErrors() {
				result = append(result, rr.AsError())
			}
		}

		if isMap {
			val.SetMapIndex(reflect.ValueOf(param.Name), target)
		}
	}

	if len(result) > 0 {
		return errors.CompositeValidationError(result...)
	}

	return nil
}

// SetLogger allows for injecting a logger to catch debug entries.
//
// The logger is enabled in DEBUG mode only.
func (o *UntypedRequestBinder) SetLogger(lg logger.Logger) {
	o.debugLogf = debugLogfFunc(lg)
}

func (o *UntypedRequestBinder) setDebugLogf(fn func(string, ...any)) {
	o.debugLogf = fn
}
