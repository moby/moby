page_title: Getting started with Docker.io
page_description: Introductory guide to getting an account on Docker.io
page_keywords: documentation, docs, the docker guide, docker guide, docker, docker platform, virtualization framework, docker.io, central service, services, how to, container, containers, automation, collaboration, collaborators, registry, repo, repository, technology, github webhooks, trusted builds

# Getting Started with Docker.io

*How do I use Docker.io?*

In this section we're going to introduce you, very quickly!, to
[Docker.io](https://index.docker.io) and create an account.

[Docker.io](https://www.docker.io) is the central hub for Docker. It
helps you to manage Docker and its components. It provides services such
as:

* Hosting images.
* User authentication.
* Automated image builds and work flow tools like build triggers and web
  hooks.
* Integration with GitHub and BitBucket.

Docker.io helps you collaborate with colleagues and get the most out of
Docker.

In order to use Docker.io you will need to register an account. Don't
panic! It's totally free and really easy.

## Creating a Docker.io Account

There are two ways you can create a Docker.io account:

* Via the web, or
* Via the command line.

### Sign up via the web!

Fill in the [sign-up form](https://www.docker.io/account/signup/) and
choose your user name and specify some details such as an email address.

![Register using the sign-up page](/userguide/register-web.png)

### Signup via the command line

You can also create a Docker.io account via the command line using the
`docker login` command.

    $ sudo docker login

### Confirm your email

Once you've filled in the form then check your email for a welcome
message and activate your account.

![Confirm your registration](/userguide/register-confirm.png)

### Login!

Then you can login using the web console:

![Login using the web console](/userguide/login-web.png)

Or via the command line and the `docker login` command:

    $ sudo docker login

Now your Docker.io account is active and ready for you to use!

##  Next steps

Now let's start Dockerizing applications with our "Hello World!" exercise.

Go to [Dockerizing Applications](/userguide/dockerizing).

