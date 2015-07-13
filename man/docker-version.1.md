% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2015
# NAME
docker-version - Show the Docker version information.

# SYNOPSIS
**docker version**

# DESCRIPTION
This command displays version information for both the Docker client and 
daemon. 

# OPTIONS
There are no available options.

# EXAMPLES

## Display Docker version information

Here is a sample output:

    $ docker version
	Client:
	 Version:      1.8.0
	 API version:  1.20
	 Go version:   go1.4.2
	 Git commit:   f5bae0a
	 Built:        Tue Jun 23 17:56:00 UTC 2015
	 OS/Arch:      linux/amd64

	Server:
	 Version:      1.8.0
	 API version:  1.20
	 Go version:   go1.4.2
	 Git commit:   f5bae0a
	 Built:        Tue Jun 23 17:56:00 UTC 2015
	 OS/Arch:      linux/amd64
	
# HISTORY
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
June 2015, updated by John Howard <jhoward@microsoft.com>
