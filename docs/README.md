# Docker Documentation

The source for Docker documentation is here under `sources/` and uses extended
Markdown, as implemented by [MkDocs](http://mkdocs.org).

The HTML files are built and hosted on
[http://docs.docker.com](http://docs.docker.com), and update automatically
after each change to the `docs` branch of [Docker on
GitHub](https://github.com/docker/docker) thanks to post-commit hooks.

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
    docker run --rm -it  -e AWS_S3_BUCKET -p 8000:8000 "docker-docs:master" mkdocs serve
    Running at: http://0.0.0.0:8000/
    Live reload enabled.
    Hold ctrl+c to quit.

If you have any issues you need to debug, you can use `make docs-shell` and then
run `mkdocs serve`

## Testing the links

You can use `make docs-test` to generate a report of missing links that are referenced in
the documentation - there should be none.

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

| Branch   | Description                    | URL (published via commit-hook)                                              |
|----------|--------------------------------|------------------------------------------------------------------------------|
| `docs`   | Official release documentation | [http://docs.docker.com](http://docs.docker.com)                             |
| `master` | Unreleased development work    | [http://docs.master.dockerproject.com](http://docs.master.dockerproject.com) |

**There are two branches related to editing docs**: `master` and `docs`. You
should always edit the documentation on a local branch of the `master` branch,
and send a PR against `master`.  That way your fixes will automatically get
included in later releases, and docs maintainers can easily cherry-pick your
changes into the `docs` release branch.  In the rare case where your change is
not forward-compatible, you may need to base your changes on the `docs` branch.

Also, since there is a separate `docs` branch, we can keep
[http://docs.docker.com](http://docs.docker.com) up to date with any bugs found
between Docker code releases.

## Publishing Documentation

To publish a copy of the documentation you need to have Docker up and running on
your machine. You'll also need a `docs/awsconfig` file containing the settings
you need to access the AWS bucket you'll be deploying to.

The release script will create an s3 if needed, and will then push the files to it.

    [profile dowideit-docs]
    aws_access_key_id = IHOIUAHSIDH234rwf....
    aws_secret_access_key = OIUYSADJHLKUHQWIUHE......
    region = ap-southeast-2

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

    make AWS_S3_BUCKET=docs.docker.com BUILD_ROOT=yes DISTRIBUTION_ID=C2K6......FL2F docs-release

Test the uncached version of the live docs at http://docs.docker.com.s3-website-us-east-1.amazonaws.com/
    
Note that the new docs will not appear live on the site until the cache (a complex,
distributed CDN system) is flushed. The `make docs-release` command will do this
_if_ the `DISTRIBUTION_ID` is set to the Cloudfront distribution ID (ask the meta
team) - this will take at least 15 minutes to run and you can check its progress
with the CDN Cloudfront Chrome addin.

## Removing files from the docs.docker.com site

Sometimes it becomes necessary to remove files from the historical published documentation.
The most reliable way to do this is to do it directly using `aws s3` commands running in a
docs container:

Start the docs container like `make docs-shell`, but bind mount in your `awsconfig`:

```
docker run --rm -it -v $(CURDIR)/docs/awsconfig:/docs/awsconfig docker-docs:master bash
```

and then the following example shows deleting 2 documents from s3, and then requesting the
CloudFlare cache to invalidate them:


```
export BUCKET BUCKET=docs.docker.com
export AWS_CONFIG_FILE=$(pwd)/awsconfig
aws s3 --profile $BUCKET ls s3://$BUCKET
aws s3 --profile $BUCKET rm s3://$BUCKET/v1.0/reference/api/docker_io_oauth_api/index.html
aws s3 --profile $BUCKET rm s3://$BUCKET/v1.1/reference/api/docker_io_oauth_api/index.html

aws configure set preview.cloudfront true
export DISTRIBUTION_ID=YUTIYUTIUTIUYTIUT
aws cloudfront  create-invalidation --profile docs.docker.com --distribution-id $DISTRIBUTION_ID --invalidation-batch '{"Paths":{"Quantity":1, "Items":["/v1.0/reference/api/docker_io_oauth_api/"]},"CallerReference":"6Mar2015sventest1"}'
aws cloudfront  create-invalidation --profile docs.docker.com --distribution-id $DISTRIBUTION_ID --invalidation-batch '{"Paths":{"Quantity":1, "Items":["/v1.1/reference/api/docker_io_oauth_api/"]},"CallerReference":"6Mar2015sventest1"}'
```

