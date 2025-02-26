# Contributing

All contributions are welcome and appreciated! Check the [Table of contents](#table-of-contents) to explore different ways to get involved and learn how contributions are handled. Before submitting, please review the relevant section to ensure a smooth process for both maintainers and contributors.

> [!TIP]
> Looking for more developer docs? Head to [docs](/docs)

## Code of conduct

The project and all participants are expected to uphold the project
[Code of Conduct](https://github.com/docker/code-of-conduct).

## I have a question

Before asking a question, be sure to search the project for existing information
to avoid duplicates and find relevant discussions. GitHub can be used to search
existing [code](https://github.com/search?q=repo%3Amoby%2Fmoby&type=code),
[issues](https://github.com/search?q=repo%3Amoby%2Fmoby&type=issues),
[pull requests](https://github.com/search?q=repo%3Amoby%2Fmoby&type=pullrequests)
and
[discussions](https://github.com/search?q=repo%3Amoby%2Fmoby&type=discussions).

> [!TIP]
> The official [engine docs](https://docs.docker.com/engine/) and [API reference](https://docs.docker.com/reference/api/engine/) are also great resources.

## I want to contribute

First -- **thank you** for taking the time to contribute! :heart:

Be sure to read the most suitable section for the type of contribution
you'd like to make. Be sure to complete all essential tasks like configuring
commit singing.

> [!TIP] 
> If you don't have time to code, consider helping with triage. The
> community will thank you for saving them time by spending some of yours.

## I want to report a security issue

The Moby maintainers take security seriously. If you discover a security
issue, please bring it to their attention right away!

> [!IMPORTANT]
> Please **DO NOT** file a public issue, instead send your report privately to
> [security@docker.com](mailto:security@docker.com).

Security reports are greatly appreciated and we will publicly thank you for it,
although we keep your name confidential if you request it. We also like to send
gifts -- if you're into schwag, make sure to let us know. We currently do not
offer a paid security bounty program, but are not ruling it out in the future.

## I want to report a bug

A great way to contribute to the project is to send a detailed report when you
encounter an issue. We always appreciate a well-written, thorough bug report,
and will thank you for it!

Check that [our issue database](https://github.com/moby/moby/issues)
doesn't already include that problem or suggestion before submitting an issue.
If you find a match, you can use the "subscribe" button to get notified on
updates. Do *not* leave random "+1" or "I have this too" comments, as they
only clutter the discussion, and don't help resolving it. However, if you
have ways to reproduce the issue or have additional information that may help
resolving the issue, please leave a comment.

When submitting an issue please provide all requested information in the form.
This information helps us review and respond to your issue faster. When sending
lengthy log-files, consider posting them as a gist (https://gist.github.com).

> [!IMPORTANT]
> Don't forget to remove sensitive data from your logfiles before posting (you can
> replace those parts with "REDACTED").

## I want to suggest an enhancement

You can propose new designs for existing Moby features. You can also design
entirely new features. We really appreciate contributors who want to refactor or
otherwise cleanup our project.

## Pull requests are always welcome

Not sure if that typo is worth a pull request? Found a bug and know how to fix
it? Do it! We will appreciate it. Any significant improvement should be
documented as [a GitHub issue](https://github.com/moby/moby/issues) before
anybody starts working on it. We are always thrilled to receive pull requests.
We do our best to process them quickly.

## Submitting a pull request

Fork the repository and make changes on your fork in a feature branch:

Submit tests for your changes. See [TESTING.md](./TESTING.md) for details.

If your changes need integration tests, write them against the API. The `cli`
integration tests are slowly either migrated to API tests or moved away as unit
tests in `docker/cli` and end-to-end tests for Docker.

Write clean code. Universally formatted code promotes ease of writing, reading,
and maintenance. Always run `gofmt -s -w file.go` on each changed file before
committing your changes. Most editors have plug-ins that do this automatically.

Pull request descriptions should be as clear as possible and include a reference
to all the issues that they address.

## Making successful changes

Before contributing large or high impact changes, make the effort to coordinate
with the maintainers of the project before submitting a pull request. This
prevents you from doing extra work that may or may not be merged.

Large PRs that are just submitted without any prior communication are unlikely
to be successful.

While pull requests are the methodology for submitting changes to code, changes
are much more likely to be accepted if they are accompanied by additional
engineering work. While we don't define this explicitly, most of these goals
are accomplished through communication of the design goals and subsequent
solutions. Often times, it helps to first state the problem before presenting
solutions.

Typically, the best methods of accomplishing this are to submit an issue,
stating the problem. This issue can include a problem statement and a
checklist with requirements. If solutions are proposed, alternatives should be
listed and eliminated. Even if the criteria for elimination of a solution is
frivolous, say so.

Larger changes typically work best with design documents. These are focused on
providing context to the design at the time the feature was conceived and can
inform future documentation contributions.

## Building and testing the project

Ready to get started? Head to [docs/contributing](./docs/contributing/) for
guides on setting up your development environment, running tests, and
contributing to the project. 🚀

## Commit messages

Commit messages must start with a capitalized and short summary (max. 50 chars)
written in the imperative, followed by an optional, more detailed explanatory
text which is separated from the summary by an empty line.

Commit messages should follow best practices, including explaining the context
of the problem and how it was solved, including in caveats or follow up changes
required. They should tell the story of the change and provide readers
understanding of what led to it.

If you're lost about what this even means, please see [How to Write a Git
Commit Message](http://chris.beams.io/posts/git-commit/) for a start.

In practice, the best approach to maintaining a nice commit message is to
leverage a `git add -p` and `git commit --amend` to formulate a solid
changeset. This allows one to piece together a change, as information becomes
available.

If you squash a series of commits, don't just submit that. Re-write the commit
message, as if the series of commits was a single stroke of brilliance.

That said, there is no requirement to have a single commit for a PR, as long as
each commit tells the story. For example, if there is a feature that requires a
package, it might make sense to have the package in a separate commit then have
a subsequent commit that uses it.

Remember, you're telling part of the story with the commit message. Don't make
your chapter weird.

## Commit signing

This project requires a Developer Certificate of Origin (DCO) sign-off on every
pull request. This is a simple statement at the end of your commit message,
certifying that you have the right to contribute the code under the project’s
[LICENSE](/LICENSE).

> [!NOTE]
> Use your real name (sorry, no pseudonyms or anonymous contributions.)

#### What does sign-off mean?

By signing off, you certify that you agree with the [Developer Certificate of Origin](http://developercertificate.org/).

<details>
<summary>Developer Certificate of Origin</summary>

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
1 Letterman Drive
Suite D4700
San Francisco, CA, 94129

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

</details>

#### How to sign a commit

To add a DCO sign-off, include the following line at the end of your commit message:

```
Signed-off-by: Joe Smith <joe.smith@email.com>
```

You can have `git` automatically add this with the `user.name` and `user.email`
from your git config by using:

```bash
git commit -s
```

> [!TIP]
> GPG signing is _highly recommended_, to configure GPG signing you can follow [GitHub's GPG guide](https://docs.github.com/en/authentication/managing-commit-signature-verification).

----

Want to hack on the Moby Project? Awesome! We have a contributor's guide that explains
[setting up a development environment and the contribution
process](docs/contributing/).

For detailed setup or advanced contribution guidance head to the
[docs/contributing](https://github.com/moby/moby/tree/master/docs/contributing)
area.

## Code review

Code review comments may be added to your pull request. Discuss, then make the
suggested modifications and push additional commits to your feature branch. Post
a comment after pushing. New commits show up in the pull request automatically,
but the reviewers are notified only when you comment.

Pull requests must be cleanly rebased on top of master without multiple branches
mixed into the PR.

> [!TIP] 
> If your PR no longer merges cleanly, use `rebase master` in your
feature branch to update your pull request rather than `merge master`.

Before you make a pull request, squash your commits into logical units of work
using `git rebase -i` and `git push -f`. A logical unit of work is a consistent
set of patches that should be reviewed together: for example, upgrading the
version of a vendored dependency and taking advantage of its now available new
feature constitute two separate units of work. Implementing a new function and
calling it in another file constitute a single logical unit of work. The very
high majority of submissions should have a single commit, so if in doubt: squash
down to one.

- After every commit, [make sure the test suite passes](./TESTING.md). Include
documentation changes in the same pull request so that a revert would remove
all traces of the feature or fix.
- Include an issue reference like `Closes #XXXX` or `Fixes #XXXX` in commits that
close an issue. Including references automatically closes the issue on a merge.
- Please do not add yourself to the `AUTHORS` file, as it is regenerated regularly
from the Git history.
- Please see the [Coding Style](#coding-style) for further guidelines.

### Merge approval

Moby maintainers use LGTM (Looks Good To Me) in comments on the code review to
indicate acceptance, or use the Github review approval feature.

## Coding Style

Unless explicitly stated, we follow all coding guidelines from the Go
community. While some of these standards may seem arbitrary, they somehow seem
to result in a solid, consistent codebase.

It is possible that the code base does not currently comply with these
guidelines. We are not looking for a massive PR that fixes this, since that
goes against the spirit of the guidelines. All new contributions should make a
best effort to clean up and make the code base better than they left it.
Obviously, apply your best judgement. Remember, the goal here is to make the
code base easier for humans to navigate and understand. Always keep that in
mind when nudging others to comply.

The rules:

1. All code should be formatted with `gofmt -s`.
2. All code should pass the default levels of
   [`golint`](https://github.com/golang/lint).
3. All code should follow the guidelines covered in [Effective
   Go](http://golang.org/doc/effective_go.html) and [Go Code Review
   Comments](https://github.com/golang/go/wiki/CodeReviewComments).
4. Comment the code. Tell us the why, the history and the context.
5. Document _all_ declarations and methods, even private ones. Declare
   expectations, caveats and anything else that may be important. If a type
   gets exported, having the comments already there will ensure it's ready.
6. Variable name length should be proportional to its context and no longer.
   `noCommaALongVariableNameLikeThisIsNotMoreClearWhenASimpleCommentWouldDo`.
   In practice, short methods will have short variable names and globals will
   have longer names.
7. No underscores in package names. If you need a compound name, step back,
   and re-examine why you need a compound name. If you still think you need a
   compound name, lose the underscore.
8. No utils or helpers packages. If a function is not general enough to
   warrant its own package, it has not been written generally enough to be a
   part of a util package. Just leave it unexported and well-documented.
9. All tests should run with `go test` and outside tooling should not be
   required. No, we don't need another unit testing framework. Assertion
   packages are acceptable if they provide _real_ incremental value.
10. Even though we call these "rules" above, they are actually just
    guidelines. Since you've read all the rules, you now know that.

If you are having trouble getting into the mood of idiomatic Go, we recommend
reading through [Effective Go](https://go.dev/doc/effective_go). The
[Go Blog](https://go.dev/blog/) is also a great resource.

### Connect with other Moby Project contributors

__Slack__

Register for the [Docker Community Slack](https://dockr.ly/comm-slack) and head
to the [#moby-project](https://dockercommunity.slack.com/archives/C50QFMRC2)
channel for general discussion.

__GitHub Discussions__

We use GitHub Discussions as a public forum for questions, design discussions
and announcements
