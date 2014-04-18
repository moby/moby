page_title: Trusted Builds in the Docker Index
page_description: Docker Index Trusted Builds
page_keywords: Docker, docker, index, accounts, plans, Dockerfile, Docker.io, docs, documentation, trusted, builds, trusted builds

# Trusted Builds in the Docker Index

## Trusted Builds

*Trusted Builds* is a special feature allowing you to specify a source
repository with a *Dockerfile* to be built by the Docker build clusters. The
system will clone your repository and build the Dockerfile using the repository
as the context. The resulting image will then be uploaded to the index and
marked as a `Trusted Build`.

Trusted Builds have a number of advantages. For example, users of *your* Trusted
Build can be certain that the resulting image was built exactly how it claims
to be.

Furthermore, the Dockerfile will be available to anyone browsing your repository
on the Index. Another advantage of the Trusted Builds feature is the automated
builds. This makes sure that your repository is always up to date.

### Linking with a GitHub account

In order to setup a Trusted Build, you need to first link your Docker Index
account with a GitHub one. This will allow the Docker Index to see your
repositories.

> *Note:* We currently request access for *read* and *write* since the Index
> needs to setup a GitHub service hook. Although nothing else is done with
> your account, this is how GitHub manages permissions, sorry!

### Creating a Trusted Build

You can [create a Trusted Build](https://index.docker.io/builds/github/select/)
from any of your public GitHub repositories with a Dockerfile.

> **Note:** We currently only support public repositories. To have more than
> one Docker image from the same GitHub repository, you will need to set up one
> Trusted Build per Dockerfile, each using a different image name. This rule
> applies to building multiple branches on the same GitHub repository as well.

### GitHub organizations

GitHub organizations appear once your membership to that organization is
made public on GitHub. To verify, you can look at the members tab for your
organization on GitHub.

### GitHub service hooks

You can follow the below steps to configure the GitHub service hooks for your
Trusted Build:

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

### The Dockerfile and Trusted Builds

During the build process, we copy the contents of your Dockerfile. We also
add it to the Docker Index for the Docker community to see on the repository
page.

### README.md

If you have a `README.md` file in your repository, we will use that as the
repository's full description.

> **Warning:**
> If you change the full description after a build, it will be
> rewritten the next time the Trusted Build has been built. To make changes,
> modify the README.md from the Git repository. We will look for a README.md
> in the same directory as your Dockerfile.

### Build triggers

If you need another way to trigger your Trusted Builds outside of GitHub, you
can setup a build trigger. When you turn on the build trigger for a Trusted
Build, it will give you a URL to which you can send POST requests. This will
trigger the Trusted Build process, which is similar to GitHub webhooks.

> **Note:** 
> You can only trigger one build at a time and no more than one
> every five minutes. If you have a build already pending, or if you already
> recently submitted a build request, those requests *will be ignored*.
> You can find the logs of last 10 triggers on the settings page to verify
> if everything is working correctly.

### Repository links

Repository links are a way to associate one Trusted Build with another. If one
gets updated, linking system also triggers a build for the other Trusted Build.
This makes it easy to keep your Trusted Builds up to date.

To add a link, go to the settings page of a Trusted Build and click on
*Repository Links*. Then enter the name of the repository that you want have
linked.

> **Warning:**
> You can add more than one repository link, however, you should
> be very careful. Creating a two way relationship between Trusted Builds will
> cause a never ending build loop.
