# Contributing

When contributing to this repository, please first discuss the change you wish
to make via an [issue](https://github.com/sigstore/timestamp-authority/issues).

## Pull Request Process

1. Create an [issue](https://github.com/sigstore/timestamp-authority/issues)
   outlining the fix or feature.
2. Fork the {project-name} repository to your own github account and clone it locally.
3. Hack on your changes.
4. Update the README.md with details of changes to any interface, this includes new environment
   variables, exposed ports, useful file locations, CLI parameters and
   new or changed configuration values.
5. Correctly format your commit message see [Commit Messages](#commit-message-guidelines)
   below.
6. Ensure that CI passes, if it fails, fix the failures.
7. Every pull request requires a review from the [core timestamp-authority team](https://github.com/orgs/github.com/sigstore/teams/tsa-codeowners)
   before merging.
8. If your pull request consists of more than one commit, please squash your
   commits as described in [Squash Commits](#squash-commits)

## Commit Message Guidelines

We follow the commit formatting recommendations found on [Chris Beams' How to Write a Git Commit Message article](https://chris.beams.io/posts/git-commit/).

Well formed commit messages not only help reviewers understand the nature of
the Pull Request, but also assists the release process where commit messages
are used to generate release notes.

A good example of a commit message would be as follows:

```text
Summarize changes in around 50 characters or less

More detailed explanatory text, if necessary. Wrap it to about 72
characters or so. In some contexts, the first line is treated as the
subject of the commit and the rest of the text as the body. The
blank line separating the summary from the body is critical (unless
you omit the body entirely); various tools like `log`, `shortlog`
and `rebase` can get confused if you run the two together.

Explain the problem that this commit is solving. Focus on why you
are making this change as opposed to how (the code explains that).
Are there side effects or other unintuitive consequences of this
change? Here's the place to explain them.

Further paragraphs come after blank lines.

 - Bullet points are okay, too

 - Typically a hyphen or asterisk is used for the bullet, preceded
   by a single space, with blank lines in between, but conventions
   vary here

If you use an issue tracker, put references to them at the bottom,
like this:

Resolves: #123
See also: #456, #789
```

Note the `Resolves #123` tag, this references the issue raised and allows us to
ensure issues are associated and closed when a pull request is merged.

Please refer to [the github help page on message types](https://help.github.com/articles/closing-issues-using-keywords/) for a complete list of issue references.

## Squash Commits

Should your pull request consist of more than one commit (perhaps due to
a change being requested during the review cycle), please perform a git squash
once a reviewer has approved your pull request.

A squash can be performed as follows. Let's say you have the following commits:

```text
initial commit
second commit
final commit
```

Run the command below with the number set to the total commits you wish to
squash (in our case 3 commits):

```shell
git rebase -i HEAD~3
```

You default text editor will then open up and you will see the following:

```shell
pick eb36612 initial commit
pick 9ac8968 second commit
pick a760569 final commit

# Rebase eb1429f..a760569 onto eb1429f (3 commands)
```

We want to rebase on top of our first commit, so we change the other two commits
to `squash`:

```shell
pick eb36612 initial commit
squash 9ac8968 second commit
squash a760569 final commit
```

After this, should you wish to update your commit message to better summarise
all of your pull request, run:

```shell
git commit --amend
```

You will then need to force push (assuming your initial commit(s) were posted
to github):

```shell
git push origin your-branch --force
```

Alternatively, a core member can squash your commits within Github.

## DCO Signoff

Make sure to sign the [Developer Certificate of
Origin](https://git-scm.com/docs/git-commit#Documentation/git-commit.txt---signoff).

## Code of Conduct

Sigstore Timestamp-Authority adheres to and enforces the [Contributor Covenant](http://contributor-covenant.org/version/1/4/) Code of Conduct.
Please take a moment to read the [CODE_OF_CONDUCT.md](/CODE_OF_CONDUCT.md) document.
