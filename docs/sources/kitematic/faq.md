page_title: Kitematic: Frequently Asked Questions
page_description: Documentation covering common questions users have about Kitematic
page_keywords: docker, documentation, about, technology, kitematic, gui


# Frequently Asked Questions

### Is Kitematic Open Source?

Yes! Our source code is available on
[GitHub](https://github.com/kitematic/kitematic). Kitematic is open source
software released under the Apache 2.0 license.

### How can I contribute to Kitematic?

We always welcome (and deeply appreciate!) new contributions to the project. The
best way to start contributing to Kitematic is to review our doc on
[contributing](https://github.com/kitematic/kitematic/blob/master/CONTRIBUTING.md).

### How does Kitematic work with Docker?

Kitematic connects directly do a running instance of Docker and controls it via
the Docker Remote API.

### Which platforms does Kitematic support?

Right now Kitematic only works on Mac OS X. That said, Windows is on the
short-term
[roadmap](https://github.com/kitematic/kitematic/blob/master/ROADMAP.md) (coming
soon!) and a Linux version is in high demand.

### Why does Kitematic collect usage analytics and bug reports?

Kitematic tracks anonymous errors and analytics to help understand why things go
wrong and to help understand how users are interacting with the app so we can
continuously make it better.

You can opt-out of this anytime via the in-app preferences.

#### What we DON'T collect

- Personal information: any information that would allow us to determine a
  specific user of Kitematic
- Information or data relating to code, containers or Docker images opened via
  Kitematic.

#### What we DO collect

- Anonymous events for actions in the app. We never collect data associated with
  events. For example:
  - User searched for images (but not what the search query was).
  - User created a container (but not which image, the name of the container or
    any data involved)
  - User opened the preferences pane
  - User deleted a container
- Errors names, messages & stack traces (scrubbed for user names)
- Operating System, Kitematic and installed VirtualBox versions

We'd love to answer any more questions about this. Feel free to reach us at
kitematic@docker.com or to open an issue on GitHub.

## Next Steps

For information about known issues in the current release of Kitematic, take a
look at the [Known issues](./known-issues.md).
