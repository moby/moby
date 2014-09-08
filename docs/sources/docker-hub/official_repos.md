page_title: Guidelines for Official Repositories on Docker Hub
page_description: Guidelines for Official Repositories on Docker Hub
page_keywords: Docker, docker, registry, accounts, plans, Dockerfile, Docker Hub, docs, official, image, documentation

# Introduction

You’ve been given the job of creating an image for an Official Repository hosted on
[Docker Hub Registry](https://registry.hub.docker.com/). These are Docker, Inc.’s
guidelines for getting that task done. Even if you’re not planning to create an Official
Repo, you can think of these guidelines as best practices for image creation generally.

This document consists of three major sections:

* Expected files, resources and supporting items for your image
* Examples embodying those practices
* Instructions for submitting contributions and reporting issues

# Expected Files & Resources

## A Git repository

Your image needs to live in a Git repository, preferably on GitHub. (If you’d like to use
a different provider, please [contact us](TODO: link) directly.) Docker **strongly**
recommends that this repo be publicly accessible.

If the repo is private or has otherwise limited access, you must provide a means of at
least “read-only” access for both general users and for the docker-library maintainers,
who need access for review and building purposes.

## A `Dockerfile`

Complete information on `Dockerfile`s can be found in the [Reference section](https://docs.docker.com/reference/builder/).
We also have a page discussing best practices for writing `Dockerfile`s (TODO: link).
Your `Dockerfile` should adhere to the following:

* It must be written either by using `FROM scratch` or be based on another, established
Official Image.
* It must follow `Dockerfile` best practices. These are discussed in the [Best Practices
document](TODO: link). In addition, Docker, Inc. engineer Michael Crosby has a good
discussion of Dockerfiles in this [blog post](http://crosbymichael.com/dockerfile-best-practices-take-2.html).

While `[ONBUILD triggers]`(https://docs.docker.com/reference/builder/#onbuild) are not
required, if you choose to use them you should:

* Build both `ONBUILD` and non-`ONBUILD` images, with the `ONBUILD` image built `FROM`
the non-`ONBUILD` image.
* The `ONBUILD` image should be specifically tagged, for example, `ruby:latest` and
`ruby:onbuild`, or `ruby:2` and  `ruby:2-onbuild`.

## A short description

Include a brief description of your image (in plaintext). Only one description is
required; you don’t need additional descriptions for each tag. The file should also: 

* Be named `README-short.txt`
* Reside in the repo for the “latest” tag
* Not exceed 200 characters.

## A logo

Include a logo of your company or the product (png format preferred). Only one logo is
required; you don’t need additional logo files for each tag. The logo file should have
the following characteristics: 

* Be named `logo.png`
* Should reside in the repo for the “latest” tag
* Should be 200px min. in one dimension, 200px max. in the other.
* Square or wide (landscape) is preferred over tall (portrait), but exceptions can be
made based on the logo needed.

## A long description

Include a comprehensive description of your image (in markdown format). Only one
description is required; you don’t need additional descriptions for each tag. The file
should also: 

* Be named `README.md`
* Reside in the repo for the “latest” tag
* Be no longer than absolutely necessary, while still addressing all the content
requirements.

In terms of content, the long description must include the following sections:

* Overview & Links
* How-to/Usage
* User Feedback
* License

### Overview & links

A section providing (a) an overview of the software contained in the image, similar to
the introduction in a Wikipedia entry and (b) a selection of links to outside resources
that help to describe the software.

### How-to/usage

A section that describes how to run and use the image, including common use cases and
example `Dockerfile`s (if applicable). Try to provide clear, step-by-step instructions
wherever possible.

### User Feedback

This section should have two parts, one explaining how users can contribute to the repo
and one explaining how to report issues with the repo.

#### Contributing

In this part, point users to any resources that can help them contribute to the project.
Include contribution guidelines and any specific instructions related to your development
practices. Include a link to [Docker’s resources for contributors](https://docs.docker.com/contributing/contributing/).
Be sure to include contact info, handles, etc. for official maintainers.

#### Issues

Include a brief section letting users know where they can go for help and how they can
file issues with the repo. Point them to any specific IRC channels, issue trackers,
contacts, additional “how-to” information or other resources.

## License

Include a file, `LICENSE`, of any applicable license.  Docker recommends using the
license of the software contained in the image, provided it allows Docker, Inc. to
legally build and distribute the image.  Otherwise Docker recommends adopting the
[Expat license]((http://directory.fsf.org/wiki/License:Expat).

# Examples

Below are sample short and long description files for an imaginary image containing
Ruby on Rails.

## Short description

     README-short.txt
    
    Ruby on Rails is an open-source application framework written in Ruby. It emphasizes
    best practices such as convention over configuration, active record pattern, and the
    model-view-controller pattern.

## Long description

    README.md
    
    # What is Ruby on Rails
    
     Ruby on Rails, often simply referred to as Rails, is an open source web application
    framework which runs via the Ruby programming language. It is a full-stack framework:
    it allows creating pages and applications that gather information from the web server,
    talk to or query the database, and render templates out of the box. As a result, Rails
    features a routing system that is independent of the web server.
    
     [wikipedia.org/wiki/Ruby_on_Rails](https://en.wikipedia.org/wiki/Ruby_on_Rails)
    
    **How to use this image**
    
    1. create a `Dockerfile` in your rails app project
    
    FROM rails:onbuild
    
    Put this file in the root of your app, next to the `Gemfile`.

    This image includes multiple `ONBUILD` triggers so that should be all that you need
    for most applications. The build will `ADD . /usr/src/app`, `RUN bundle install`,
    `EXPOSE 3000`, and set the default command to `rails server`.
    
    2. build the rails app image
    
    docker build -t my-rails-app .
    
    3. start the rails app container
    
    docker run --name some-rails-app -d my-rails-app
    
     Then go to `http://container-ip:3000` in a browser. On the other hand, if you need access
     outside the host on port 8080:
    
    docker run --name some-rails-app -p 8080:3000 -d my-rails-app
    
    Then go to `http://localhost:8080` or `http://host-ip:8080` in a browser.

