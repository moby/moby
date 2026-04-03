## `pathrs-lite` ##

`github.com/cyphar/filepath-securejoin/pathrs-lite` provides a minimal **pure
Go** implementation of the core bits of [libpathrs][]. This is not intended to
be a complete replacement for libpathrs, instead it is mainly intended to be
useful as a transition tool for existing Go projects.

The long-term plan for `pathrs-lite` is to provide a build tag that will cause
all `pathrs-lite` operations to call into libpathrs directly, thus removing
code duplication for projects that wish to make use of libpathrs (and providing
the ability for software packagers to opt-in to libpathrs support without
needing to patch upstream).

[libpathrs]: https://github.com/cyphar/libpathrs

### License ###

Most of this subpackage is licensed under the Mozilla Public License (version
2.0). For more information, see the top-level [COPYING.md][] and
[LICENSE.MPL-2.0][] files, as well as the individual license headers for each
file.

```
Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
Copyright (C) 2024-2025 SUSE LLC

This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at https://mozilla.org/MPL/2.0/.
```

[COPYING.md]: ../COPYING.md
[LICENSE.MPL-2.0]: ../LICENSE.MPL-2.0
