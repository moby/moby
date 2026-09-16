/*
*
* Copyright 2026 gRPC authors.
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
 */

// Package internal contains functionality internal to the google package.
package internal

import (
	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials/idtoken"
	"google.golang.org/grpc/internal/backoff"
)

// The following variables are overridden in tests.
var (
	// BackoffStrategy is the backoff strategy to use when token fetch fails.
	BackoffStrategy backoff.Strategy

	// NewIDTokenCredentials builds idtoken credentials using specified options.
	NewIDTokenCredentials func(opts *idtoken.Options) (*auth.Credentials, error)
)
