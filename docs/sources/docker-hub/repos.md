page_title: Repositories and Images on Docker Hub
page_description: Repositories and Images on Docker Hub
page_keywords: Docker, docker, registry, accounts, plans, Dockerfile, Docker Hub, docs, documentation

# Repositories and Images on Docker Hub

![repositories](/docker-hub/repos.png)

## Searching for repositories and images

You can `search` for all the publicly available repositories and images using
Docker.

    $ docker search ubuntu

This will show you a list of the currently available repositories on the
Docker Hub which match the provided keyword.

If a repository is private it won't be listed on the repository search
results. To see repository statuses, you can look at your [profile
page](https://hub.docker.com) on [Docker Hub](https://hub.docker.com).

## Repositories

Your Docker Hub repositories have a number of useful features.

### Stars

Your repositories can be starred and you can star repositories in
return. Stars are a way to show that you like a repository. They are
also an easy way of bookmarking your favorites.

### Comments

You can interact with other members of the Docker community and maintainers by
leaving comments on repositories. If you find any comments that are not
appropriate, you can flag them for review.

### Collaborators and their role

A collaborator is someone you want to give access to a private
repository. Once designated, they can `push` and `pull` to your
repositories. They will not be allowed to perform any administrative
tasks such as deleting the repository or changing its status from
private to public.

> **Note:**
> A collaborator cannot add other collaborators. Only the owner of
> the repository has administrative access.

You can also collaborate on Docker Hub with organizations and groups.
You can read more about that [here](accounts/).

## Official Repositories

The Docker Hub contains a number of [official
repositories](http://registry.hub.docker.com/official). These are
certified repositories from vendors and contributors to Docker. They
contain Docker images from vendors like Canonical, Oracle, and Red Hat
that you can use to build applications and services.

If you use Official Repositories you know you're using a supported,
optimized and up-to-date image to power your applications.

> **Note:**
> If you would like to contribute an official repository for your
> organization, product or team you can see more information
> [here](https://github.com/docker/stackbrew).

## Private Repositories

Private repositories allow you to have repositories that contain images
that you want to keep private, either to your own account or within an
organization or group.

To work with a private repository on [Docker
Hub](https://hub.docker.com), you will need to add one via the [Add
Repository](https://registry.hub.docker.com/account/repositories/add/)
link. You get one private repository for free with your Docker Hub
account. If you need more accounts you can upgrade your [Docker
Hub](https://registry.hub.docker.com/plans/) plan.

Once the private repository is created, you can `push` and `pull` images
to and from it using Docker.

> *Note:* You need to be signed in and have access to work with a
> private repository.

Private repositories are just like public ones. However, it isn't
possible to browse them or search their content on the public registry.
They do not get cached the same way as a public repository either.

It is possible to give access to a private repository to those whom you
designate (i.e., collaborators) from its Settings page. From there, you
can also switch repository status (*public* to *private*, or
vice-versa). You will need to have an available private repository slot
open before you can do such a switch. If you don't have any available,
you can always upgrade your [Docker
Hub](https://registry.hub.docker.com/plans/) plan.

## Webhooks

You can configure webhooks for your repositories on the Repository
Settings page. A webhook is called only after a successful `push` is
made. The webhook calls are HTTP POST requests with a JSON payload
similar to the example shown below.

> **Note:** For testing, you can try an HTTP request tool like
> [requestb.in](http://requestb.in/).

*Example webhook JSON payload:*

    {
       "push_data":{
          "pushed_at":1385141110,
          "images":[
             "imagehash1",
             "imagehash2",
             "imagehash3"
          ],
          "pusher":"username"
       },
       "repository":{
          "status":"Active",
          "description":"my docker repo that does cool things",
          "is_automated":false,
          "full_description":"This is my full description",
          "repo_url":"https://registry.hub.docker.com/u/username/reponame/",
          "owner":"username",
          "is_official":false,
          "is_private":false,
          "name":"reponame",
          "namespace":"username",
          "star_count":1,
          "comment_count":1,
          "date_created":1370174400,
          "dockerfile":"my full dockerfile is listed here",
          "repo_name":"username/reponame"
       }
    }

Webhooks allow you to notify people, services and other applications of
new updates to your images and repositories.

