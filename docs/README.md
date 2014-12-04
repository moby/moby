# Docker Documentation

The source for Docker documentation is here under `sources/` and uses extended
Markdown, as implemented by [MkDocs](http://mkdocs.org).

The HTML files are built and hosted on `https://docs.docker.com`, and update
automatically after each change to the master or release branch of [Docker on
GitHub](https://github.com/docker/docker) thanks to post-commit hooks. The
`docs` branch maps to the "latest" documentation and the `master` (unreleased
development) branch maps to the "master" documentation.

## Contributing

Be sure to follow the [contribution guidelines](../CONTRIBUTING.md).
In particular, [remember to sign your work!](../CONTRIBUTING.md#sign-your-work)

## Getting Started

Docker documentation builds are done in a Docker container, which installs all
the required tools, adds the local `docs/` directory and builds the HTML docs.
It then starts a HTTP server on port 8000 so that you can connect and see your
changes.

In the root of the `docker` source directory:

    $ make docs
    .... (lots of output) ....
    $ docker run --rm -it  -e AWS_S3_BUCKET -p 8000:8000 "docker-docs:master" mkdocs serve
    Running at: http://0.0.0.0:8000/
    Live reload enabled.
    Hold ctrl+c to quit.

If you have any issues you need to debug, you can use `make docs-shell` and then
run `mkdocs serve`

## Adding a new document

New document (`.md`) files are added to the documentation builds by adding them
to the menu definition in the `docs/mkdocs.yml` file.

## Style guide

If you have questions about how to write for Docker's documentation (e.g.,
questions about grammar, syntax, formatting, styling, language, or tone) please
see the [style guide](sources/contributing/docs_style-guide.md). If something
isn't clear in the guide, please submit a PR to help us improve it.

## Working using GitHub's file editor

Alternatively, for small changes and typos you might want to use GitHub's built-
in file editor. It allows you to preview your changes right on-line (though
there can be some differences between GitHub Markdown and [MkDocs
Markdown](http://www.mkdocs.org/user-guide/writing-your-docs/)).  Just be
careful not to create many commits. And you must still [sign your
work!](../CONTRIBUTING.md#sign-your-work)

## Branches

**There are two branches related to editing docs**: `master` and `docs`. You
should always edit the documentation on a local branch of the `master`
branch, and send a PR against `master`.

That way your fixes will automatically get included in later releases, and docs
maintainers can easily cherry-pick your changes into the `docs` release branch.
In the rare case where your change is not forward-compatible, you may need to
base your changes on the `docs` branch.

Also, now that we have a `docs` branch, we can keep the
[http://docs.docker.com](http://docs.docker.com) docs up to date with any bugs
found between Docker code releases.

> **Warning**: When *reading* the docs, the
> [http://docs-stage.docker.com](http://docs-stage.docker.com) documentation may
> include features not yet part of any official Docker release. The `beta-docs`
> site should be used only for understanding bleeding-edge development and
> `docs.docker.com` (which points to the `docs` branch`) should be used for the
> latest official release.

## Publishing Documentation

To publish a copy of the documentation you need to have Docker up and running on
your machine. You'll also need a `docs/awsconfig` file containing the settings
you need to access the AWS bucket you'll be deploying to.

The release script will create an s3 if needed, and will then push the files to it.

    [profile dowideit-docs] aws_access_key_id = IHOIUAHSIDH234rwf....
    aws_secret_access_key = OIUYSADJHLKUHQWIUHE......  region = ap-southeast-2

The `profile` name must be the same as the name of the bucket you are deploying
to - which you call from the `docker` directory:

    make AWS_S3_BUCKET=dowideit-docs docs-release

This will publish _only_ to the `http://bucket-url/v1.2/` version of the
documentation.

If you're publishing the current release's documentation, you need to
also update the root docs pages by running

    make AWS_S3_BUCKET=dowideit-docs BUILD_ROOT=yes docs-release

> **Note:**
> if you are using Boot2Docker on OSX and the above command returns an error,
> `Post http:///var/run/docker.sock/build?rm=1&t=docker-docs%3Apost-1.2.0-docs_update-2:
> dial unix /var/run/docker.sock: no such file or directory', you need to set the Docker
> host. Run `$(boot2docker shellinit)` to see the correct variable to set. The command
> will return the full `export` command, so you can just cut and paste.

## Cherry-picking documentation changes to update an existing release.

Whenever the core team makes a release, they publish the documentation based
on the `release` branch (which is copied into the `docs` branch). The
documentation team can make updates in the meantime, by cherry-picking changes
from `master` into any of the docs branches.

For example, to update the current release's docs:

    git fetch upstream
    git checkout -b post-1.2.0-docs-update-1 upstream/docs
    # Then go through the Merge commit linked to PR's (making sure they apply
    to that release)
    # see https://github.com/docker/docker/commits/master
    git cherry-pick -x fe845c4
    # Repeat until you have cherry picked everything you will propose to be merged
    git push upstream post-1.2.0-docs-update-1

Then make a pull request to merge into the `docs` branch, __NOT__ into master.

Once the PR has the needed `LGTM`s, merge it, then publish to our beta server
to test:

    git fetch upstream
    git checkout docs
    git reset --hard upstream/docs
    make AWS_S3_BUCKET=beta-docs.docker.io BUILD_ROOT=yes docs-release

Then go to http://beta-docs.docker.io.s3-website-us-west-2.amazonaws.com/
to view your results and make sure what you published is what you wanted.

When you're happy with it, publish the docs to our live site:

    make AWS_S3_BUCKET=docs.docker.com BUILD_ROOT=yes docs-release

Test the uncached version of the live docs at http://docs.docker.com.s3-website-us-east-1.amazonaws.com/
    
Note that the new docs will not appear live on the site until the cache (a complex,
distributed CDN system) is flushed. This requires someone with S3 keys. Contact Docker
(Sven Dowideit or John Costa) for assistance. 

