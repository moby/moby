page_title: Installation on Google Cloud Platform
page_description: Please note this project is currently under heavy development. It should not be used in production.
page_keywords: Docker, Docker documentation, installation, google, Google Compute Engine, Google Cloud Platform

# Google Cloud Platform

> **Note**:
> Docker is still under heavy development! We don't recommend using it in
> production yet, but we're getting closer with each release. Please see
> our blog post, [Getting to Docker 1.0](
> http://blog.docker.io/2013/08/getting-to-docker-1-0/)

## Compute Engine QuickStart for debian

1. Go to [Google Cloud Console](https://cloud.google.com/console) and
   create a new Cloud Project with [Compute Engine
   enabled](https://developers.google.com/compute/docs/signup).

2. Download and configure the [Google Cloud SDK](
   https://developers.google.com/cloud/sdk/) to use your project
   with the following commands:

    ```
    $ curl https://dl.google.com/dl/cloudsdk/release/install_google_cloud_sdk.bash | bash
    $ gcloud auth login
    Enter a cloud project id (or leave blank to not set): <google-cloud-project-id>
    ...
    ```

3. Start a new instance using the latest `container-vm-*` image (select a zone close to you and the desired instance size):

    ```
    $ gcloud compute instances create docker-playground
      --image projects/google-containers/global/images/container-vm-v20140522
      --zone us-central1-a
      --machine-type f1-micro
    ```

4. Connect to the instance using SSH

    ```
    $ gcloud compute ssh --zone us-central1-a docker-playground
    docker-playground:~$ sudo docker run busybox echo 'docker on GCE \o/'
    docker on GCE \o/
    ```

Read more about [deploying Containers on Google Cloud Platform](https://developers.google.com/compute/docs/containers).
