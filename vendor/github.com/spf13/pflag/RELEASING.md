# Releasing pflag

This document describes how to create a new pflag release.

## Method 1: GitHub UI (recommended by maintainers)

1. Go to the [releases page](https://github.com/spf13/pflag/releases)
2. Click **"Draft a new release"**
3. Click **"Generate release notes"** — this shows the merged PRs since the last tag, which helps determine the next version (minor vs patch)
4. Choose the tag version (e.g., `v1.0.8`) — the tag will be created when the release is published
5. Review the generated notes, then publish

This method has the advantage that the tag and release are published atomically, and the PR list helps pick the right version number before tagging.

## Method 2: GitHub CLI

```bash
git checkout master
git pull origin master

# Tag and push
git tag v1.0.8
git push origin v1.0.8

# Create release with auto-generated notes
gh release create v1.0.8 --generate-notes
```

## Notes

pflag follows [Semantic Versioning](https://semver.org/). Check the [releases page](https://github.com/spf13/pflag/releases) for the latest tag.
