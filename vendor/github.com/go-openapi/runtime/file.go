// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import "github.com/go-openapi/swag/fileutils"

// File represents an uploaded file. Re-exported from
// [fileutils.File] for backwards compatibility.
//
// See [BindForm] (in form.go) for the orchestrator that parses
// multipart / urlencoded request bodies and binds declared file
// fields onto handler-side targets.
type File = fileutils.File
