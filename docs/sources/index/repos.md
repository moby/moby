page_title: Repositories and Images in the Docker Index
page_description: Docker Index repositories
page_keywords: Docker, docker, index, accounts, plans, Dockerfile, Docker.io, docs, documentation

# Repositories and Images in the Docker Index

## Searching for repositories and images

You can `search` for all the publicly available repositories and images using
Docker. If a repository is not public (i.e., private), it won't be listed on
the Index search results. To see repository statuses, you can look at your
[profile page](https://index.docker.io/account/).

## Repositories

### Stars

Stars are a way to show that you like a repository. They are also an easy way
of bookmark your favorites.

### Comments

You can interact with other members of the Docker community and maintainers by
leaving comments on repositories. If you find any comments that are not
appropriate, you can flag them for the Index admins' review.

### Private Docker Repositories

To work with a private repository on the Docker Index, you will need to add one
via the [Add Repository](https://index.docker.io/account/repositories/add) link.
Once the private repository is created, you can `push` and `pull` images to and
from it using Docker.

> *Note:* You need to be signed in and have access to work with a private
> repository.

Private repositories are just like public ones. However, it isn't possible to
browse them or search their content on the public index. They do not get cached
the same way as a public repository either.

It is possible to give access to a private repository to those whom you 
designate (i.e., collaborators) from its settings page.

From there, you can also switch repository status (*public* to *private*, or
viceversa). You will need to have an available private repository slot open
before you can do such a switch. If you don't have any, you can always upgrade
your [Docker Index plan](https://index.docker.io/plans/).

### Collaborators and their role

A collaborator is someone you want to give access to a private repository. Once
designated, they can `push` and `pull`. Although, they will not be allowed to
perform any administrative tasks such as deleting the repository or changing its
status from private to public.

> **Note:** A collaborator can not add other collaborators. Only the owner of
> the repository has administrative access.

### Webhooks

You can configure webhooks on the repository settings page. A webhook is called
only after a successful `push` is made. The webhook calls are HTTP POST requests
with a JSON payload similar to the example shown below.

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
          "is_trusted":false,
          "full_description":"This is my full description",
          "repo_url":"https://index.docker.io/u/username/reponame/",
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
