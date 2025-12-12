# Patch (bugfix) release process

The patch releases can be cut straight from the `master` branch if there are no
changes that would warrant a non-patch release.

However, if the `master` branch has changes that are not suitable for a patch
release, a release branch should be created.

See [BRANCHES-AND-TAGS.md](BRANCHES-AND-TAGS.md) for more information on the release branches.

## Backporting changes

If a release branch exists, changes that are not suitable for a patch release should be backported to the release branch.

A patch must:

- Not be a major/new feature
- Not break existing functionality
- Be a bugfix or a small improvement

To indicate that a pull request made against the `master` branch should be
included in the next patch release, the author or maintainer should apply a
`process/cherry-pick/<BRANCH_NAME>` label corresponding to the release branch.
