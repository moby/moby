docker pull feature
===================

Allow username/passwords for a Docker registry to be passed to the Docker client via CLI flags or in image url.

Example:
```
docker pull -u <username> -p <password> registry.com/devx/my_image
docker pull  <username>:<password>@registry.com/devx/my_image
```

#Why?
**Automation**:  No longer requires a human to do a ‘docker login’, and would allow for the system to retrieve the image on it’s own.

**Concurrency**: Multiple users with different usernames and possibly different organizations (within an enterprise org) utilizing docker pull at the same time.

**Security**:    No need to store passwords on the system and no need for a user to provide their specific username and password (assuming robot accounts are implemented and used)

**Isolation**: Allow for the use of robot accounts per repo. i.e. we wouldn’t need 1 robot account that has access to all private images.
