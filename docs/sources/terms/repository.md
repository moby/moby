Repository[¶](#repository "Permalink to this headline")
=======================================================

A repository is a set of images either on your local Docker server, or
shared, by pushing it to a [*Registry*](../registry/#registry-def)
server.

Images can be associated with a repository (or multiple) by giving them
an image name using one of three different commands:

1.  At build time (e.g. `sudo docker build -t IMAGENAME`{.docutils
    .literal}),
2.  When committing a container (e.g.
    `sudo docker commit CONTAINERID IMAGENAME`{.docutils .literal}) or
3.  When tagging an image id with an image name (e.g.
    `sudo docker tag IMAGEID IMAGENAME`{.docutils .literal}).

A Fully Qualified Image Name (FQIN) can be made up of 3 parts:

`[registry_hostname[:port]/][user_name/](repository_name[:version_tag])`{.docutils
.literal}

`version_tag`{.docutils .literal} defaults to `latest`{.docutils
.literal}, `username`{.docutils .literal} and
`registry_hostname`{.docutils .literal} default to an empty string. When
`registry_hostname`{.docutils .literal} is an empty string, then
`docker push`{.docutils .literal} will push to
`index.docker.io:80`{.docutils .literal}.

If you create a new repository which you want to share, you will need to
set at least the `user_name`{.docutils .literal}, as the ‘default’ blank
`user_name`{.docutils .literal} prefix is reserved for official Docker
images.

For more information see [*Working with
Repositories*](../../use/workingwithrepository/#working-with-the-repository)
