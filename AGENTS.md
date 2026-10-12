# Commits

Keep each commit focused on one atomic, independently reviewable change.
A pull request may contain multiple commits when they represent separate changes.
Fold review fixes into the commits they correct before merging, rather than leaving a separate commit for each review fix.
Temporary fixup commits are fine during review, but squash them into the appropriate commits before merging.
Prefer separate commits for unrelated changes rather than squashing them just to make the pull request a single commit.

# Pull requests

When opening a pull request:

- Follow `.github/PULL_REQUEST_TEMPLATE.md` and fill in the applicable sections.
- Do not add `Co-Authored-By` trailers for LLMs or AI agents; reserve them for
  human contributors.
- Add "Created with: <AGENT NAME>" note at the end of the PR body.

## Release note section

If the change is worthy putting in a user-facing release changelog, write a brief entry, ideally one sentence.
Use present tense and imperative tone.
Draw inspiration from past release notes: https://docs.docker.com/engine/release-notes/
