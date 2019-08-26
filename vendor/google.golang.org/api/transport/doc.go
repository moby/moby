// Copyright 2019 Google LLC
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

// Package transport provides utility methods for creating authenticated
// transports to Google's HTTP and gRPC APIs. It is intended to be used in
// conjunction with google.golang.org/api/option.
//
// This package is not intended for use by end developers. Use the
// google.golang.org/api/option package to configure API clients.
package transport
