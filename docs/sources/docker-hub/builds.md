page_title: Automated Builds on Docker Hub
page_description: Docker Hub Automated Builds
page_keywords: Docker, docker, registry, accounts, plans, Dockerfile, Docker Hub, docs, documentation, trusted, builds, trusted builds, automated builds
# Automated Builds on Docker Hub

## About Automated Builds

*Automated Builds* are a special feature of Docker Hub which allow you to use
[Docker Hub's](https://hub.docker.com) build clusters to automatically create images from
a specified `Dockerfile` and a GitHub or Bitbucket repo (or "context"). The system will
clone your repository and build the image described by the `Dockerfile` using the
repository as the context. The resulting automated image will then be uploaded to the
Docker Hub registry and marked as an *Automated Build*.

Automated Builds have several advantages:

* Users of *your* Automated Build can trust that the resulting image was built exactly as
specified.

* The `Dockerfile` will be available to anyone with access to your repository
on the Docker Hub registry. 

* Because the process is automated, Automated Builds help to make sure that your
repository is always up to date.

Automated Builds are supported for both public and private repositories on both
[GitHub](http://github.com) and [Bitbucket](https://bitbucket.org/).

To use Automated Builds, you must have an 
[account on Docker Hub](http://docs.docker.com/userguide/dockerhub/#creating-a-docker-hub-account)
and on GitHub and/or Bitbucket.

## Setting up Automated Builds with GitHub

In order to set up an Automated Build, you need to first link your
[Docker Hub](https://hub.docker.com) account with a GitHub account. This
will allow the registry to see your repositories.

> *Note:* 
> Automated Builds currently require *read* and *write* access since
> [Docker Hub](https://hub.docker.com) needs to setup a GitHub service
> hook. We have no choice here, this is how GitHub manages permissions, sorry! 
> We do guarantee nothing else will be touched in your account.

To get started, log into your Docker Hub account and click the "+ Add Repository" button
at the upper right of the screen. Then select
[Automated Build](https://registry.hub.docker.com/builds/add/).

Select the [GitHub service](https://registry.hub.docker.com/associate/github/).

Then follow the onscreen instructions to authorize and link your GitHub account to Docker Hub.

### Creating an Automated Build

You can [create an Automated Build](https://registry.hub.docker.com/builds/github/select/)
from any of your public or private GitHub repositories with a `Dockerfile`.

### GitHub organizations

GitHub organizations will appear once your membership to that organization is
made public on GitHub. To verify, you can look at the members tab for your
organization on GitHub.

### GitHub service hooks

Follow the steps below to configure the GitHub service hooks for your
Automated Build:

<table class="table table-bordered">
  <thead>
    <tr>
      <th>Step</th>
      <th>Screenshot</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>1.</td>
      <td><img src="https://d207aa93qlcgug.cloudfront.net/0.8/img/github_settings.png"></td>
      <td>Log in to Github.com, and visit your Repository page. Click on repository "Settings" on the right side of the page. You must have admin privileges to the repository in order to do this.</td>
    </tr>
    <tr>
      <td>2.</td>
      <td><img src="https://d207aa93qlcgug.cloudfront.net/0.8/img/github_service_hooks.png" alt="Service Hooks"></td>
      <td>Click on "Webhooks & Services" on the left side of the page.</td></tr><tr><td>3.</td><td><img src="https://d207aa93qlcgug.cloudfront.net/0.8/img/github_docker_service_hook.png" alt="Find the service labeled Docker"></td><td>Find the service labeled "Docker" and click on it.</td></tr><tr><td>4.</td><td><img src="https://d207aa93qlcgug.cloudfront.net/0.8/img/github_service_hook_docker_activate.png" alt="Activate Service Hooks"></td>
      <td>Make sure the "Active" checkbox is selected and click the "Update service" button to save your changes.</td>
    </tr>
  </tbody>
</table>

## Setting up Automated Builds with Bitbucket

In order to setup an Automated Build, you need to first link your
[Docker Hub](https://hub.docker.com) account with a Bitbucket account. This
will allow the registry to see your repositories.

To get started, log into your Docker Hub account and click the "+ Add Repository" button at
the upper right of the screen. Then select [Automated Build](https://registry.hub.docker.com/builds/add/).

Select the [Bitbucket
service](https://registry.hub.docker.com/associate/bitbucket/).

Then follow the onscreen instructions to authorize and link your Bitbucket account
to Docker Hub.

### Creating an Automated Build

You can [create an Automated Build](
https://registry.hub.docker.com/builds/bitbucket/select/) from any of your
public or private Bitbucket repositories with a `Dockerfile`.

## The Dockerfile and Automated Builds

During the build process, we copy the contents of your `Dockerfile`. We also
add it to the [Docker Hub](https://hub.docker.com) for the Docker community (for public
repos) or approved team members/orgs (for private repos) to see on the repository page
(if your repo is public).

## README.md

If you have a `README.md` file in your repository, it will be used as the
repository's full description.

> **Warning:**
> If you change the full description after a build, it will be
> rewritten the next time the Automated Build has been built. To make changes,
> modify the `README.md` from the Git repository. The build process will look for a
> `README.md` in the same directory as your `Dockerfile`.

### Build triggers

If you need a way to trigger Automated Builds outside of GitHub
or Bitbucket, you can set up a build trigger. When you turn on the build
trigger for an Automated Build, it will give you a URL to which you can
send POST requests. This will trigger the Automated Build, which
is similar to GitHub webhooks.

Build triggers are available under the Settings tab of each Automated Build.

> **Note:** 
> You can only trigger one build at a time and no more than one
> every five minutes. If you have a build already pending, or if you already
> recently submitted a build request, those requests *will be ignored*.
> You can find the logs of last 10 triggers on the settings page to verify
> if everything is working correctly.

### Webhooks

Also available for Automated Builds are Webhooks. Webhooks can be called
after a successful repository push is made.

The webhook call will generate a HTTP POST with the following JSON
payload:

```
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
```

Webhooks are available under the Settings tab of each Automated
Build.

> **Note:** If you want to test your webhook out then we recommend using
> a tool like [requestb.in](http://requestb.in/).


### Repository links

Repository links are a way to associate one Automated Build with another. If one
gets updated, linking system also triggers a build for the other Automated Build.
This makes it easy to keep your Automated Builds up to date.

To add a link, go to the settings page of an Automated Build and click on
*Repository Links*. Then enter the name of the repository that you want have
linked.

> **Warning:**
> You can add more than one repository link, however, you should
> be very careful. Creating a two way relationship between Automated Builds will
> cause a never ending build loop.
