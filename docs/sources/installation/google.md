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

1. Go to [Google Cloud Console](https://cloud.google.com/console) and create a new Cloud Project with [Compute Engine enabled](https://developers.google.com/compute/docs/signup).
2. Download and configure the [Google Cloud SDK](https://developers.google.com/cloud/sdk/) to use your project with the following commands:

	```
	$ curl https://dl.google.com/dl/cloudsdk/release/install_google_cloud_sdk.bash | bash
	$ gcloud auth login
	Your browser has been opened to visit:

	https://accounts.google.com/o/oauth2/auth?scope=https%3A%2F%2Fwww.googleapis.co%2
	Fauth%2Fappengine.admin+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fbigquery+https
	%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcompute+https%3A%2F%2Fwww.googleapis.com%
	Fauth%2Fdevstorage.full_control+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuser...

	Created new window in existing browser session.
	```

3. `gloud` will do one of the following
	1. Open your default browser with the URL (with the above output). This can be overridden with the `--no-launch-browser` flag.
	2. Output a clickable link, if your terminal supports it.
	3. Require you to copy/paste the full url into a browser. In this case, the above output will instead be `Go to the following link in your browser:`, followed by the URL.
4. If you have multiple Google account logins cached in your browser, select the account associated with your GCP account from the list, or add it. Click "Accept" to give Google Cloud SDK permission to your GCP account. If your terminal application supports it the resulting code will be pasted to your prompt automatically. If not, the next page will present:

	```
	Please copy this code, switch to your application and paste it there:
	<Unique-62-char-random-string-oauth2-response>
	```

5. Switch back to your terminal, and if the output was not automatically redirected, paste in the code

	```
	Enter verification code:<Unique-62-char-random-string-oauth2-response>
	You can view your existing projects and create new ones in the Google Developers Console at: https://console.developers.google.com. If you have a project ready, you can enter it now.
	Enter a cloud project id (or leave blank to not set): <your-google-project-id>
	```
> **Note**;
> Be sure to enter the auto-generated Project ID, as the given name for a GCE project name can be changed at will and 
> does not map to the ID.

6.  Start a new instance, select a zone close to you and the desired instance size:

	```
	$ gcutil addinstance docker-playground --image=backports-debian-7
	1: europe-west1-a
	...
	4: us-central1-b
	>>> <zone-index>
	1: machineTypes/n1-standard-1
	...
	12: machineTypes/g1-small
	>>> <machine-type-index>
	```

7. Connect to the instance using SSH:

	```
	$ gcutil ssh docker-playground
	docker-playground:~$
	```    
> **Note**;
> Google discourages logging into GCE instances as root, but you can override this/subdue the warning by setting the `--permit_root_ssh` flag in the above command.

8. Install the latest Docker release and configure it to start when the instance boots:

	```
	docker-playground:~$ curl get.docker.io | bash
	docker-playground:~$ sudo update-rc.d docker defaults
	```

9. Start a new container:

	```
	docker-playground:~$ sudo docker run busybox echo 'docker on GCE \o/'
	docker on GCE \o/
	```
