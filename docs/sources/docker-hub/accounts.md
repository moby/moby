page_title: Accounts on Docker Hub
page_description: Docker Hub accounts
page_keywords: Docker, docker, registry, accounts, plans, Dockerfile, Docker Hub, docs, documentation

# Accounts on Docker Hub

## Docker Hub accounts

You can `search` for Docker images and `pull` them from [Docker
Hub](https://hub.docker.com) without signing in or even having an
account. However, in order to `push` images, leave comments or to *star*
a repository, you are going to need a [Docker
Hub](https://hub.docker.com) account.

### Registration for a Docker Hub account

You can get a [Docker Hub](https://hub.docker.com) account by
[signing up for one here](https://hub.docker.com/account/signup/). A valid
email address is required to register, which you will need to verify for
account activation.

### Email activation process

You need to have at least one verified email address to be able to use your
[Docker Hub](https://hub.docker.com) account. If you can't find the validation email,
you can request another by visiting the [Resend Email Confirmation](
https://hub.docker.com/account/resend-email-confirmation/) page.

### Password reset process

If you can't access your account for some reason, you can reset your password
from the [*Password Reset*](https://hub.docker.com/account/forgot-password/)
page.

## Organizations and groups

A Docker Hub organization contains public and private repositories just like
a user account. Access to push, pull or create these organisation owned repositories
is allocated by defining groups of users and then assigning group rights to
specific repositories. This allows you to distribute limited access
Docker images, and to select which Docker Hub users can publish new images.

### Creating and viewing organizations

You can see what organizations [you belong to and add new organizations](
https://hub.docker.com/account/organizations/) from the Account Settings
tab. They are also listed below your user name on your repositories page
and in your account profile.

![organizations](/docker-hub/hub-images/orgs.png)

### Organization groups

Users in the `Owners` group of an organization can create and modify the
membership of groups.

Unless they are the organization's `Owner`, users can only see groups of which they
are members.

![groups](/docker-hub/hub-images/groups.png)

### Repository group permissions

Use organization groups to manage who can interact with your repositories.

You need to be a member of the organization's `Owners` group to create a new group,
Hub repository or automated build. As an `Owner`, you then delegate the following
repository access rights to groups:

- `Read` access allows a user to view, search, and pull a private repository in the
  same way as they can a public repository.
- `Write` access users are able to push to non-automated repositories on the Docker
  Hub.
- `Admin` access allows the user to modify the repositories "Description", "Collaborators" rights,
  "Mark as unlisted", "Public/Private" status and "Delete".

> **Note**: A User who has not yet verified their email address will only have
> `Read` access to the repository, regardless of the rights their group membership
>  gives them.

![Organization repository collaborators](/docker-hub/hub-images/org-repo-collaborators.png)


