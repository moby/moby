# Docker Documentation

The source for Docker documentation is here under `sources/` and uses extended
Markdown, as implemented by [MkDocs](http://mkdocs.org).

The HTML files are built and hosted on `https://docs.docker.com`, and update
automatically after each change to the master or release branch of [Docker on
GitHub](https://github.com/docker/docker) thanks to post-commit hooks. The
`docs` branch maps to the "latest" documentation and the `master` (unreleased
development) branch maps to the "master" documentation.

## Branches

**There are two branches related to editing docs**: `master` and a `docs`
branch. You should always edit documentation on a local branch of the `master`
branch, and send a PR against `master`.

That way your fixes will automatically get included in later releases, and docs
maintainers can easily cherry-pick your changes into the `docs` release branch.
In the rare case where your change is not forward-compatible, you may need to
base your changes on the `docs` branch.

Also, now that we have a `docs` branch, we can keep the
[http://docs.docker.com](http://docs.docker.com) docs up to date with any bugs
found between Docker code releases.

**Warning**: When *reading* the docs, the
[http://docs-stage.docker.com](http://docs-stage.docker.com) documentation may
include features not yet part of any official Docker release. The `beta-docs`
site should be used only for understanding bleeding-edge development and
`docs.docker.com` (which points to the `docs` branch`) should be used for the
latest official release.

## Contributing

- Follow the contribution guidelines ([see
  `../CONTRIBUTING.md`](../CONTRIBUTING.md)).
- [Remember to sign your work!](../CONTRIBUTING.md#sign-your-work)

## Getting Started

Docker documentation builds are done in a Docker container, which installs all
the required tools, adds the local `docs/` directory and builds the HTML docs.
It then starts a HTTP server on port 8000 so that you can connect and see your
changes.

In the root of the `docker` source directory:

    make docs

If you have any issues you need to debug, you can use `make docs-shell` and then
run `mkdocs serve`

## Style guide

The documentation is written with paragraphs wrapped at 80 column lines to make
it easier for terminal use.

### Examples

When writing examples, give the user hints by making them resemble what they see
in their shell:

- Indent shell examples by 4 spaces so they get rendered as code.
- Start typed commands with `$ ` (dollar space), so that they are easily
  differentiated from program output.
- Program output has no prefix.
- Comments begin with `# ` (hash space).
- In-container shell commands begin with `$$ ` (dollar dollar space).

### Images

When you need to add images, try to make them as small as possible (e.g., as
gifs). Usually images should go in the same directory as the `.md` file which
references them, or in a subdirectory if one already exists.

## Working using GitHub's file editor

Alternatively, for small changes and typos you might want to use GitHub's built-
in file editor. It allows you to preview your changes right on-line (though
there can be some differences between GitHub Markdown and [MkDocs
Markdown](http://www.mkdocs.org/user-guide/writing-your-docs/)).  Just be
careful not to create many commits. And you must still [sign your
work!](../CONTRIBUTING.md#sign-your-work)

## Publishing Documentation

To publish a copy of the documentation you need a `docs/awsconfig`
file containing AWS settings to deploy to. The release script will
create an s3 if needed, and will then push the files to it.

    [profile dowideit-docs] aws_access_key_id = IHOIUAHSIDH234rwf....
    aws_secret_access_key = OIUYSADJHLKUHQWIUHE......  region = ap-southeast-2

The `profile` name must be the same as the name of the bucket you are deploying
to - which you call from the `docker` directory:

    make AWS_S3_BUCKET=dowideit-docs docs-release

