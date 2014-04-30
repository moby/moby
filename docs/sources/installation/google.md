page_title: Installation on Google Cloud Platform
page_description: Please note this project is currently under heavy development. It should not be used in production.
page_keywords: Docker, Docker documentation, installation, google, Google Compute Engine, Google Cloud Platform

# Google Cloud Platform

> **Note**:
> Docker is still under heavy development! We don't recommend using it in
> production yet, but we're getting closer with each release. Please see
> our blog post, [Getting to Docker 1.0](
> http://blog.docker.io/2013/08/getting-to-docker-1-0/)

## Compute Engine QuickStart for Debian

1. Go to [Google Cloud Console](https://cloud.google.com/console) and
   create a new Cloud Project with [Compute Engine
   enabled](https://developers.google.com/compute/docs/signup).
2. Download and configure the [Google Cloud SDK](
   https://developers.google.com/cloud/sdk/) to use your project
   with the following commands:

<!-- -->

    $ curl https://dl.google.com/dl/cloudsdk/release/install_google_cloud_sdk.bash | bash
    $ gcloud auth login
    Go to the following link in your browser:
      https://accounts.google.com/o/oauth2/auth?scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fappengine.admin+https%3A       %2F%2Fwww.googleapis.com%2Fauth%2Fbigquery+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcompute+https%3A%2F%2Fwww.goo       gleapiscom%2Fauth%2Fdevstorage.full_control+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.email+https%3A%2F%2       Fwww.googleapis.com%2Fauth%2Fndev.cloudman+https%3A%F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform+https%3A%2F%2Fw       ww.googleapis.com%2Fauth%2Fsqlservice.admin+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fprediction+https%3A%2F%2Fwww       .googleapis.com%2Fauth%2Fprojecthosting&redirect_uri=urn%3Aietf%3Awg%3Aoauth%3A2.0%3Aoob&response_type=code&client       _id=XXXXXXXXXXX.apps.googleusercontent.com&access_type=offline

3.   Copy/paste the full url into a browser. If you have multiple Google account logins cached in your browser, select       the account associated with your GCP account from the list, or add it. Click "Accept" to give Google Cloud SDK          permission to your GCP account. The next page will present:

<!-- -->

    Please copy this code, switch to your application and paste it there:
    <Unique-62-char-random-string-oauth2-response>
    
    Paste this code to your prompt:

<!-- -->

    Enter verification code:<Unique-62-char-random-string-oauth2-response>
    You can view your existing projects and create new ones in the Google Developers Console at:
    https://console.developers.google.com. If you have a project ready, you can enter it now.
    Enter a cloud project id (or leave blank to not set): <your-google-project-id>
> **Note**;
> Be sure to enter the auto-generated Project ID, as the given name for a GCE project name can be changed at will and 
> does not map to the ID.

4.  Start a new instance, select a zone close to you and the desired
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
> **Note**;
> If you plan on using Vagrant for Docker provisioning, do not currently use the backports-debian-7-wheezy image
  (v20140415 as of this writing), as the current Vagrant backports version (1.0.3-1) does not install Virtualbox
  properly on Wheezy

5.  Connect to the instance using SSH:

<!-- -->

    $ gcutil --service_version="v1" --project="<your-google-project-id>" ssh --zone="<your-gce-instance-zone>" 
    "docker-playground"
    docker-playground:~$
    
> **Note**;
> Google discourages logging into GCE instances as root,
> but you can override this/subdue the warning by setting 
> the `--permit_root_ssh` flag in the above command.

5.  Install the latest Docker release and configure it to start when the
    instance boots:

<!-- -->

    docker-playground:~$ curl get.docker.io | bash
    docker-playground:~$ sudo update-rc.d docker defaults

6.  Start a new container:

<!-- -->

    docker-playground:~$ sudo docker run busybox echo 'docker on GCE \o/'
    docker on GCE \o/
