# Contribute to the Moby Project

We are always thrilled to receive pull requests.
We do our best to process them quickly.
See the [contributor's guide](docs/contributing/) for development setup and the [review process](project/REVIEWING.md) for how maintainers review contributions.
Read the [AI usage policy](#ai-usage-policy) and [community guidelines](#moby-community-guidelines) before participating.

## Reporting security issues

Report vulnerabilities privately to [security@docker.com](mailto:security@docker.com), not in public issues.
See [SECURITY.md](SECURITY.md) for the reporting process and supported versions.

## Reporting other issues

Search [existing issues](https://github.com/moby/moby/issues) before opening a report.
Include reproduction steps and the output of `docker version` and `docker info`.
Remove secrets and private data from logs before sharing them.
For existing reports, subscribe for updates and add useful evidence rather than "+1" comments.

## Quick contribution tips and guidelines

### Design and cleanup proposals

Discuss significant improvements, new features, refactoring, and cleanup proposals in a [GitHub issue](https://github.com/moby/moby/issues) before starting implementation.
Explain the problem, proposed approach, alternatives, and user or maintenance benefit so maintainers can agree on the scope.
Use a design document for larger changes when needed.

### Where to put your changes

Keep changes with the component that owns them and follow the existing package layout.

- `api/` and `client/` - Independently versioned public Go modules for API types and the API client
- `cmd/` and `daemon/` - Engine entry points and implementation
- `integration/` - API integration tests
- `docs/` and `man/` - Technical documentation and man page generation
- `hack/` - Build, test, and validation tooling

The root module builds the engine and is not intended to be imported as a library; see [Go modules](README.md#go-modules).
Prefer `internal` directories for new packages that are not intended for other modules.
Keep new Go source files out of the repository root and new packages out of the legacy `pkg/` directory.
Regenerate vendored files with the repository's vendoring tooling rather than editing `vendor/` directly.

### Conventions

Keep the pull request focused and explain the problem and solution in its description.
Put related issue and PR links there, using a closing keyword such as `Fixes` when appropriate.

Include tests for your changes; see [TESTING.md](TESTING.md) for guidance and test filters.
Add new integration tests under `integration/` using the API client, not the deprecated `integration-cli/` suite.
Keep each commit passing the tests and validation relevant to its changes.
Use `make test-unit`, `make test-integration`, and `make validate` as appropriate.

Include relevant API and reference documentation updates with the code change.
User guides live in [docker/docs](https://github.com/docker/docs); follow its [style guide](https://github.com/docker/docs/blob/main/STYLE.md) and [contribution instructions](https://github.com/docker/docs/blob/main/CONTRIBUTING.md).
Check documentation for clarity, correctness, and broken links, and run the relevant validation.
The `AUTHORS` file is generated from Git history; leave it unchanged.

### Commit Messages

Use a short imperative summary, prefixed with the affected component for scoped changes, such as `daemon:` or `client:`.
Separate an optional body with a blank line and explain the problem, fix, and important constraints.
See [How to Write a Git Commit Message](https://cbea.ms/git-commit/) for more guidance.

Keep each commit focused on one atomic, independently reviewable change.
A pull request can contain multiple commits for separate logical units of work.
Prefer separate commits for unrelated changes rather than squashing them just to make the pull request a single commit.
When squashing, rewrite the message to describe the resulting change, not the intermediate steps.

CI rejects GitHub issue and PR shorthand and URLs in commit messages; put those references in the PR description or comments.
Use commit hashes when referring to other code changes in commit messages.

### Review

Discuss feedback, push the agreed changes, and reply briefly with what changed and which checks you ran.
Temporary fixup commits are fine during review, but fold review fixes into the commits they correct before merging.

Rebase on the latest target branch (usually `master`) rather than merging it into your feature branch.
Use `git rebase -i` to organize commits into logical units of work.
When updating your own remote branch after rewriting commits, use `git push --force-with-lease` to avoid overwriting others' updates.
Coordinate with collaborators before rewriting a shared branch.

Maintainers approve changes through LGTM comments or GitHub reviews; see [REVIEWING](project/REVIEWING.md) for the process.

### Sign your work

Every commit must include a `Signed-off-by` trailer certifying that you meet the [Developer Certificate of Origin](https://developercertificate.org/).
Use a known identity (sorry, no anonymous contributions.)
Configure Git's `user.name` and `user.email`, then use `git commit -s` to add the trailer:

```text
Signed-off-by: Joe Smith <joe.smith@email.com>
```

## AI usage policy

Read and follow the shared [Moby and Docker AI usage policy](https://github.com/moby/.github/blob/main/AI_POLICY.md).
Disclose which tools you used and roughly how much work they did in the issue or PR description, not in commit messages.
Review and edit AI-assisted content before posting; you remain responsible for the submission.
Reserve `Co-Authored-By` trailers for human contributors, not LLMs or AI agents.

## Moby community guidelines

- Be respectful and welcoming, regardless of background or experience; abuse is not tolerated.
- Share only content you own, respect privacy, and follow the law.
- Stay on topic and avoid spam.
- Use GitHub mentions rather than email to request maintainer review.

Connect through the [forums](https://forums.mobyproject.org), [Docker Community Slack](https://dockr.ly/comm-slack) (`#moby-project`), or [Moby Twitter feed](https://twitter.com/moby/).
See [project/GOVERNANCE.md](project/GOVERNANCE.md) for project governance.

### Guideline violations — 3 strikes method

1. First occurrence: a public reminder of the guidelines.
2. Second occurrence: a private warning that further violations will result in removal.
3. Third occurrence: possible account deletion or banning, depending on the violation.

These rules apply equally to everyone.
Violations are forgiven after six months of good behavior, and minor infractions receive guidance.
Obvious spammers are banned on the first occurrence.
Extreme threatening, abusive, destructive, or illegal behavior is addressed immediately without strikes or forgiveness.
Report abuse or appeal violations to [abuse@docker.com](mailto:abuse@docker.com); mistakes and misunderstandings can be reviewed.

## Coding Style

Follow the conventions of the package you are changing, [Effective Go](https://go.dev/doc/effective_go), and [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).
Avoid unrelated cleanup in a focused change.

- Format changed Go files with `gofmt -s -w`.
- Use the repository's [golangci-lint configuration](.golangci.yml); run `make validate-golangci-lint` to use the development container's linter.
- Document exported APIs and their important expectations and caveats.
- Explain non-obvious intent and constraints in comments rather than narrating code.
- Scale variable names to their scope and avoid underscores in package names.
- Keep code with its owning concern rather than creating generic `utils` or `helpers` packages.
- Use Go's `testing` package and `gotest.tools/v3/assert` for new tests.
  Integration tests require the repository's daemon and test harness; see [TESTING.md](TESTING.md).
