// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"net/http"

	"github.com/go-openapi/runtime"
)

// Authorized provides a default implementation of the Authorizer interface where all
// requests are authorized (successful)
func Authorized() runtime.Authorizer {
	return runtime.AuthorizerFunc(func(_ *http.Request, _ any) error { return nil })
}
