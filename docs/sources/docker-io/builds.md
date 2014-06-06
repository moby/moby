page_title: Automated Builds on Docker.io
page_description: Docker.io Automated Builds
page_keywords: Docker, docker, registry, accounts, plans, Dockerfile, Docker.io, docs, documentation, trusted, builds, trusted builds, automated, automated builds
# Automated Builds on Docker.io

## Automated Builds

*Automated Builds* is a special feature allowing you to specify a source
repository with a `Dockerfile` to be built by the
[Docker.io](https://index.docker.io) build clusters. The system will
clone your repository and build the `Dockerfile` using the repository as
the context. The resulting image will then be uploaded to the registry
and marked as an *Automated Build*.

Automated Builds have a number of advantages. For example, users of
*your* Automated Build can be certain that the resulting image was built
exactly how it claims to be.

Furthermore, the `Dockerfile` will be available to anyone browsing your repository
on the registry. Another advantage of the Automated Builds feature is the automated
builds. This makes sure that your repository is always up to date.

Automated Builds are supported for both public and private repositories
on both [GitHub](http://github.com) and
[BitBucket](https://bitbucket.org/).

### Setting up Automated Builds with GitHub

In order to setup an Automated Build, you need to first link your [Docker.io](
https://index.docker.io) account with a GitHub one. This will allow the registry
to see your repositories.

> *Note:* We currently request access for *read* and *write* since [Docker.io](
> https://index.docker.io) needs to setup a GitHub service hook. Although nothing
> else is done with your account, this is how GitHub manages permissions, sorry!

Click on the [Automated Builds tab](https://index.docker.io/builds/) to
get started and then select [+ Add
New](https://index.docker.io/builds/add/).

Select the [GitHub
service](https://index.docker.io/associate/github/).

Then follow the instructions to authorize and link your GitHub account
to Docker.io.

#### Creating an Automated Build

You can [create an Automated Build](https://index.docker.io/builds/github/select/)
from any of your public or private GitHub repositories with a `Dockerfile`.

#### GitHub organizations

GitHub organizations appear once your membership to that organization is
made public on GitHub. To verify, you can look at the members tab for your
organization on GitHub.

#### GitHub service hooks

You can follow the below steps to configure the GitHub service hooks for your
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
      <td>Login to Github.com, and visit your Repository page. Click on the repository "Settings" link. You will need admin rights to the repository in order to do this. So if you don't have admin rights, you will need to ask someone who does.</td>
    </tr>
    <tr>
      <td>2.</td>
      <td><img src="https://d207aa93qlcgug.cloudfront.net/0.8/img/github_service_hooks.png" alt="Service Hooks"></td>
      <td>Click on the "Service Hooks" link</td></tr><tr><td>3.</td><td><img src="https://d207aa93qlcgug.cloudfront.net/0.8/img/github_docker_service_hook.png" alt="Find the service hook labeled Docker"></td><td>Find the service hook labeled "Docker" and click on it.</td></tr><tr><td>4.</td><td><img src="https://d207aa93qlcgug.cloudfront.net/0.8/img/github_service_hook_docker_activate.png" alt="Activate Service Hooks"></td>
      <td>Click on the "Active" checkbox and then the "Update settings" button, to save changes.</td>
    </tr>
  </tbody>
</table>

### Setting up Automated Builds with BitBucket

In order to setup an Automated Build, you need to first link your
[Docker.io]( https://index.docker.io) account with a BitBucket one. This
will allow the registry to see your repositories.

Click on the [Automated Builds tab](https://index.docker.io/builds/) to
get started and then select [+ Add
New](https://index.docker.io/builds/add/).

Select the [BitBucket
service](https://index.docker.io/associate/bitbucket/).

Then follow the instructions to authorize and link your BitBucket account
to Docker.io.

#### Creating an Automated Build

You can [create an Automated
Build](https://index.docker.io/builds/bitbucket/select/) from any of
your public or private BitBucket repositories with a `Dockerfile`.

### The Dockerfile and Automated Builds

During the build process, we copy the contents of your `Dockerfile`. We also
add it to the [Docker.io](https://index.docker.io) for the Docker community
to see on the repository page.

### README.md

If you have a `README.md` file in your repository, we will use that as the
repository's full description.

> **Warning:**
> If you change the full description after a build, it will be
> rewritten the next time the Automated Build has been built. To make changes,
> modify the README.md from the Git repository. We will look for a README.md
> in the same directory as your `Dockerfile`.

### Build triggers

If you need another way to trigger your Automated Builds outside of GitHub
or BitBucket, you can setup a build trigger. When you turn on the build
trigger for an Automated Build, it will give you a URL to which you can
send POST requests. This will trigger the Automated Build process, which
is similar to GitHub webhooks.

Build Triggers are available under the Settings tab of each Automated Build.

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
