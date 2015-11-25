<!--[metadata]>
+++
title = "tag"
description = "The tag command description and usage"
keywords = ["tag, name, image"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# tag

    Usage: docker tag [OPTIONS] IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]

    Tag an image into a repository

      -f, --force=false    Force the tagging even if there's a conflict
      --help=false         Print usage

You can group your images together using names and tags, and then upload them
to [*Share Images via Repositories*](../../userguide/dockerrepos.md#contributing-to-docker-hub).
