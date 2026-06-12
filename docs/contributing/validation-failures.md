# Known validation failures

These notes cover contributor-fixable validation failures that may show up in CI for pull requests.

<a id="validate-no-gh-references"></a>
## Commit messages must not reference GitHub issues or pull requests

Commit messages in this repository must not contain references such as `#1234`, `moby/moby#1234`, or `https://github.com/moby/moby/issues/1234`.

Keep those references in GitHub-visible text instead, such as the pull request description or a comment. For example, put `Closes moby/moby#1234` in the PR description, not in a commit message.

If CI reports this failure, rewrite the affected commit messages with `git commit --amend` or `git rebase -i`, then force-push the updated branch.

<a id="validate-no-dco"></a>
## DCO sign-off is missing or malformed

Each commit must include a `Signed-off-by:` line with your real name and email address. Do not use your GitHub handle in place of your name.

The easiest way to add or fix the sign-off for the most recent commit is:

```console
git commit --amend --signoff
```

If multiple commits need to be fixed, use `git rebase -i` and amend each affected commit with `--signoff`.

See [CONTRIBUTING.md](../../CONTRIBUTING.md#sign-your-work) for the required format.

<a id="validate-module-replace"></a>
## API or client changes require temporary replace rules

Changes under `api/` or `client/` may require temporary `replace` rules in the top-level `go.mod` while the pull request is under review.

Regenerate the replace rules with:

```console
./hack/vendor.sh replace
```

Commit the resulting `go.mod` and `go.sum` updates to your branch.

<a id="validate-vendor"></a>
## Vendoring must be regenerated

When dependency metadata changes, the checked-in vendored files must be refreshed so that `go.mod`, `go.sum`, and `vendor/` stay in sync.

Re-run vendoring with:

```console
./hack/vendor.sh
```

If CI still reports vendoring problems, inspect the updated files, commit the resulting changes, and push the branch again.
