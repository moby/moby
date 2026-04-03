// SPDX-License-Identifier: MPL-2.0

//go:build linux

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package securejoin

import (
	"github.com/cyphar/filepath-securejoin/pathrs-lite"
)

var (
	// MkdirAll is a wrapper around [pathrs.MkdirAll].
	//
	// Deprecated: You should use [pathrs.MkdirAll] directly instead. This
	// wrapper will be removed in filepath-securejoin v0.6.
	MkdirAll = pathrs.MkdirAll

	// MkdirAllHandle is a wrapper around [pathrs.MkdirAllHandle].
	//
	// Deprecated: You should use [pathrs.MkdirAllHandle] directly instead.
	// This wrapper will be removed in filepath-securejoin v0.6.
	MkdirAllHandle = pathrs.MkdirAllHandle

	// OpenInRoot is a wrapper around [pathrs.OpenInRoot].
	//
	// Deprecated: You should use [pathrs.OpenInRoot] directly instead. This
	// wrapper will be removed in filepath-securejoin v0.6.
	OpenInRoot = pathrs.OpenInRoot

	// OpenatInRoot is a wrapper around [pathrs.OpenatInRoot].
	//
	// Deprecated: You should use [pathrs.OpenatInRoot] directly instead. This
	// wrapper will be removed in filepath-securejoin v0.6.
	OpenatInRoot = pathrs.OpenatInRoot

	// Reopen is a wrapper around [pathrs.Reopen].
	//
	// Deprecated: You should use [pathrs.Reopen] directly instead. This
	// wrapper will be removed in filepath-securejoin v0.6.
	Reopen = pathrs.Reopen
)
