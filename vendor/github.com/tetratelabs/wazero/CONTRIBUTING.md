# Contributing

We welcome contributions from the community. Please read the following guidelines carefully to maximize the chances of your PR being merged.

## Coding Style

- To ensure your change passes format checks, run `make check`. To format your files, you can run `make format`.
- We follow standard Go table-driven tests and use an internal [testing library](./internal/testing/require) to assert correctness. To verify all tests pass, you can run `make test`.

## DCO

We require DCO signoff line in every commit to this repo.

The sign-off is a simple line at the end of the explanation for the
patch, which certifies that you wrote it or otherwise have the right to
pass it on as an open-source patch. The rules are pretty simple: if you
can certify the below (from
[developercertificate.org](https://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1
Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA
Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.
Developer's Certificate of Origin 1.1
By making a contribution to this project, I certify that:
(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or
(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or
(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.
(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

then you just add a line to every git commit message:

    Signed-off-by: Joe Smith <joe@gmail.com>

using your real name (sorry, no pseudonyms or anonymous contributions.)

You can add the sign off when creating the git commit via `git commit -s`.

## Code Reviews

* The pull request title should describe what the change does and not embed issue numbers.
The pull request should only be blank when the change is minor. Any feature should include
a description of the change and what motivated it. If the change or design changes through
review, please keep the title and description updated accordingly.
* A single approval is sufficient to merge. If a reviewer asks for
changes in a PR they should be addressed before the PR is merged,
even if another reviewer has already approved the PR.
* During the review, address the comments and commit the changes
_without_ squashing the commits. This facilitates incremental reviews
since the reviewer does not go through all the code again to find out
what has changed since the last review. When a change goes out of sync with main,
please rebase and force push, keeping the original commits where practical.
* Commits are squashed prior to merging a pull request, using the title
as commit message by default. Maintainers may request contributors to
edit the pull request tite to ensure that it remains descriptive as a
commit message. Alternatively, maintainers may change the commit message directly.
