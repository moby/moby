page_title: Installation on Google Cloud Platform
page_description: Please note this project is currently under heavy development. It should not be used in production.
page_keywords: Docker, Docker documentation, installation, google, Google Compute Engine, Google Cloud Platform

# [Google Cloud Platform](https://cloud.google.com/)

> **Note**:
> Docker is still under heavy development! We don’t recommend using it in
> production yet, but we’re getting closer with each release. Please see
> our blog post, [Getting to Docker 1.0](
> http://blog.docker.io/2013/08/getting-to-docker-1-0/)

## [Compute Engine](https://developers.google.com/compute) QuickStart for [Debian](https://www.debian.org)

1.  Go to [Google Cloud Console](https://cloud.google.com/console) and
    create a new Cloud Project with [Compute Engine
    enabled](https://developers.google.com/compute/docs/signup).
2.  Download and configure the [Google Cloud
    SDK](https://developers.google.com/cloud/sdk/) to use your project
    with the following commands:

<!-- -->

    $ curl https://dl.google.com/dl/cloudsdk/release/install_google_cloud_sdk.bash | bash
    $ gcloud auth login
    Enter a cloud project id (or leave blank to not set): <google-cloud-project-id>

3.  Start a new instance, select a zone close to you and the desired
    instance size:

<!-- -->

    $ gcutil addinstance docker-playground --image=backports-debian-7
    1: europe-west1-a
    ...
    4: us-central1-b
    >>> <zone-index>
    1: machineTypes/n1-standard-1
    ...
    12: machineTypes/g1-small
    >>> <machine-type-index>

4.  Connect to the instance using SSH:

<!-- -->

    $ gcutil ssh docker-playground
    docker-playground:~$

5.  Install the latest Docker release and configure it to start when the
    instance boots:

<!-- -->

    docker-playground:~$ curl get.docker.io | bash
    docker-playground:~$ sudo update-rc.d docker defaults

6.  Start a new container:

<!-- -->

    docker-playground:~$ sudo docker run busybox echo 'docker on GCE \o/'
    docker on GCE \o/
