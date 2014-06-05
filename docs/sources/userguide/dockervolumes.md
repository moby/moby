page_title: Managing Data in Containers
page_description: How to manage data inside your Docker containers.
page_keywords: Examples, Usage, volume, docker, documentation, user guide, data, volumes

# Managing Data in Containers

So far we've been introduced some [basic Docker
concepts](/userguide/usingdocker/), seen how to work with [Docker
images](/userguide/dockerimages/) as well as learned about [networking
and links between containers](/userguide/dockerlinks/). In this section
we're going to discuss how you can manage data inside and between your
Docker containers.

We're going to look at the two primary ways you can manage data in
Docker.

* Data volumes, and
* Data volume containers.

## Data volumes

A *data volume* is a specially-designated directory within one or more
containers that bypasses the [*Union File
System*](/terms/layer/#ufs-def) to provide several useful features for
persistent or shared data:

- Data volumes can be shared and reused between containers
- Changes to a data volume are made directly
- Changes to a data volume will not be included when you update an image
- Volumes persist until no containers use them

### Adding a data volume

You can add a data volume to a container using the `-v` flag with the
`docker run` command. You can use the `-v` multiple times in a single
`docker run` to mount multiple data volumes. Let's mount a single volume
now in our web application container.

    $ sudo docker run -d -P --name web -v /webapp training/webapp python app.py

This will create a new volume inside a container at `/webapp`.

> **Note:** 
> You can also use the `VOLUME` instruction in a `Dockerfile` to add one or
> more new volumes to any container created from that image.

### Mount a Host Directory as a Data Volume

In addition to creating a volume using the `-v` flag you can also mount a
directory from your own host into a container.

    $ sudo docker run -d -P --name web -v /src/webapp:/opt/webapp training/webapp python app.py

This will mount the local directory, `/src/webapp`, into the container as the
`/opt/webapp` directory. This is very useful for testing, for example we can
mount our source code inside the container and see our application at work as
we change the source code. The directory on the host must be specified as an
absolute path and if the directory doesn't exist Docker will automatically
create it for you.

> **Note::** 
> This is not available from a `Dockerfile` due the portability
> and sharing purpose of it. As the host directory is, by its nature,
> host-dependent it might not work all hosts.

Docker defaults to a read-write volume but we can also mount a directory
read-only.

    $ sudo docker run -d -P --name web -v /src/webapp:/opt/webapp:ro training/webapp python app.py

Here we've mounted the same `/src/webapp` directory but we've added the `ro`
option to specify that the mount should be read-only.

## Creating and mounting a Data Volume Container

If you have some persistent data that you want to share between
containers, or want to use from non-persistent containers, it's best to
create a named Data Volume Container, and then to mount the data from
it.

Let's create a new named container with a volume to share.

    $ docker run -d -v /dbdata --name dbdata training/postgres

You can then use the `--volumes-from` flag to mount the `/dbdata` volume in another container.

    $ docker run -d --volumes-from dbdata --name db1 training/postgres

And another:

    $ docker run -d --volumes-from dbdata --name db2 training/postgres

You can use multiple `-volumes-from` parameters to bring together multiple data
volumes from multiple containers.

You can also extend the chain by mounting the volume that came from the
`dbdata` container in yet another container via the `db1` or `db2` containers.

    $ docker run -d --name db3 --volumes-from db1 training/postgres

If you remove containers that mount volumes, including the initial `dbdata`
container, or the subsequent containers `db1` and `db2`, the volumes will not
be deleted until there are no containers still referencing those volumes. This
allows you to upgrade, or effectively migrate data volumes between containers.

## Backup, restore, or migrate data volumes

Another useful function we can perform with volumes is use them for
backups, restores or migrations.  We do this by using the
`--volumes-from` flag to create a new container that mounts that volume,
like so:

    $ sudo docker run --volumes-from dbdata -v $(pwd):/backup ubuntu tar cvf /backup/backup.tar /dbdata

Here's we've launched a new container and mounted the volume from the
`dbdata` container. We've then mounted a local host directory as
`/backup`. Finally, we've passed a command that uses `tar` to backup the
contents of the `dbdata` volume to a `backup.tar` file inside our
`/backup` directory. When the command completes and the container stops
we'll be left with a backup of our `dbdata` volume.

You could then to restore to the same container, or another that you've made
elsewhere. Create a new container.

    $ sudo docker run -v /dbdata --name dbdata2 ubuntu

Then un-tar the backup file in the new container's data volume.

    $ sudo docker run --volumes-from dbdata2 -v $(pwd):/backup busybox tar xvf /backup/backup.tar

You can use this techniques above to automate backup, migration and
restore testing using your preferred tools.

# Next steps

Now we've learned a bit more about how to use Docker we're going to see how to
combine Docker with the services available on
[Docker.io](https://index.docker.io) including Automated Builds and private
repositories.

Go to [Working with Docker.io](/userguide/dockerrepos).

