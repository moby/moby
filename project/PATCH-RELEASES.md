# Patch (bugfix) release process

The patch releases can be cut straight from the `master` branch if there are no
changes that would warrant a non-patch release.

However, if the `master` branch has changes that are not suitable for a patch
release, a release branch should be created.

See [BRANCHES-AND-TAGS.md](BRANCHES-AND-TAGS.md) for more information on the release branches.

## Backporting changes

If a release branch exists (because `master` has changes that are not suitable for
a patch release), then bug fixes and patches need to be backported to that release
branch. If patches can be shipped directly from `master`, no backporting is needed.

A patch must:

- Not be a major/new feature
- Not break existing functionality
- Be a bugfix or a small improvement

To indicate that a pull request made against the `master` branch should be
included in the next patch release, the author or maintainer should apply a
`process/cherry-pick/<BRANCH_NAME>` label corresponding to the release branch.
