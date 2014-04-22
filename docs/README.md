Docker Documentation
====================

Overview
--------

The source for Docker documentation is here under ``sources/`` and uses
extended Markdown, as implemented by [mkdocs](http://mkdocs.org).

The HTML files are built and hosted on https://docs.docker.io, and update
automatically after each change to the master or release branch of the
[docker files on GitHub](https://github.com/dotcloud/docker) thanks to
post-commit hooks. The "release" branch maps to the "latest"
documentation and the "master" (unreleased development) branch maps to the "master"
documentation. 

## Branches

**There are two branches related to editing docs**: ``master`` and a
``docs`` branch. You should always edit
docs on a local branch of the ``master`` branch, and send a PR against ``master``. 
That way your fixes 
will automatically get included in later releases, and docs maintainers 
can easily cherry-pick your changes into the ``docs`` release branch. 
In the rare case where your change is not forward-compatible, 
you may need to base your changes on the ``docs`` branch.

Now that we have a ``docs`` branch, we can keep the [http://docs.docker.io](http://docs.docker.io) docs
up to date with any bugs found between ``docker`` code releases.

**Warning**: When *reading* the docs, the [http://beta-docs.docker.io](http://beta-docs.docker.io) documentation may
include features not yet part of any official docker
release. The ``beta-docs`` site should be used only for understanding
bleeding-edge development and ``docs.docker.io`` (which points to the ``docs``
branch``) should be used for the latest official release.

Getting Started
---------------

Docker documentation builds are done in a docker container, which installs all
the required tools, adds the local ``docs/`` directory and builds the HTML
docs. It then starts a HTTP server on port 8000 so that you can connect 
and see your changes.

In the ``docker`` source directory, run:
    ```make docs```

If you have any issues you need to debug, you can use ``make docs-shell`` and
then run ``mkdocs serve``

# Contributing

## Normal Case:

* Follow the contribution guidelines ([see
  ``../CONTRIBUTING.md``](../CONTRIBUTING.md)).
* [Remember to sign your work!](../CONTRIBUTING.md#sign-your-work)
* Work in your own fork of the code, we accept pull requests.
* Change the ``.md`` files with your favorite editor -- try to keep the
  lines short (80 chars) and respect Markdown conventions. 
* Run ``make clean docs`` to clean up old files and generate new ones,
  or just ``make docs`` to update after small changes.
* Your static website can now be found in the ``_build`` directory.
* To preview what you have generated run ``make server`` and open
  http://localhost:8000/ in your favorite browser.

``make clean docs`` must complete without any warnings or errors.

Working using GitHub's file editor
----------------------------------

Alternatively, for small changes and typos you might want to use
GitHub's built in file editor. It allows you to preview your changes
right online (though there can be some differences between GitHub
Markdown and mkdocs Markdown). Just be careful not to create many commits.
And you must still [sign your work!](../CONTRIBUTING.md#sign-your-work)

Images
------

When you need to add images, try to make them as small as possible
(e.g. as gif). Usually images should go in the same directory as the
.md file which references them, or in a subdirectory if one already
exists.

Publishing Documentation
------------------------

To publish a copy of the documentation you need a ``docs/awsconfig``
file containing AWS settings to deploy to. The release script will 
create an s3 if needed, and will then push the files to it.

```
[profile dowideit-docs]
aws_access_key_id = IHOIUAHSIDH234rwf....
aws_secret_access_key = OIUYSADJHLKUHQWIUHE......
region = ap-southeast-2
```

The ``profile`` name must be the same as the name of the bucket you are 
deploying to - which you call from the docker directory:

``make AWS_S3_BUCKET=dowideit-docs docs-release``

