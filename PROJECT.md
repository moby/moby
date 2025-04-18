# Project

This document outlines the project’s processes for releases, issue and pull
request management, workflow tracking, and automation, providing contributors
with the necessary guidelines for effective collaboration.

Dates, assignments, status and other data will be updated here as they change.

If this document has missing or out-of-date information please file an
[issue](https://github.com/moby/moby/issues/new/choose) or submit a pull
request.

## 🏷️ Labels

The project uses GitHub labels to communicate key metadata about issues and pull requests,
such as lifecycle state, priority, or required actions. Projects have quite a
few labels -- let's look at a few key categories used to support this one.

| Group              | Purpose                                          | Example                  |
| ------------------ | ------------------------------------------------ | ------------------------ |
| State              | To indicate the current status of an issue or PR | `state/duplicate`        |
| Focus areas        | To group items by areas of work or ownership     | `area/ci`                |
| Global identifiers | To reflect decisions or validations              | `accepted`               |
| Needs              | To list required actions                         | `needs/more-information` |
| Impact             | To indicate external or unique impacts areas     | `impact/api`             |

> [!TIP]
> There is a pinned [issue](https://github.com/moby/moby/issues) that provides more detail about labels used in the project. 

## 📝 Issues

**Issues** are used to report bugs, request enhancements, or discuss proposals.
Templates are provided to guide submissions and automatically apply information
such as type and applicable labels.

Issues will fall into one of the following types, each with a supporting template:

| Type                                                                                                                   | Description                                                              |
| ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| [Bug](https://github.com/moby/moby/issues?q=sort%3Aupdated-desc%20is%3Aissue%20is%3Aopen%20type%3ABug)                 | Reports incorrect or unexpected behavior                                 |
| [Enhancement](https://github.com/moby/moby/issues?q=sort%3Aupdated-desc%20is%3Aissue%20is%3Aopen%20type%3AEnhancement) | Suggests new functionality or enhancements to existing features          |
| [Task](https://github.com/moby/moby/issues?q=sort%3Aupdated-desc%20is%3Aissue%20is%3Aopen%20type%3ATask)               | Work that needs to be completed but is not a bug or enhancement          |
| [Epic](https://github.com/moby/moby/issues?q=sort%3Aupdated-desc%20is%3Aissue%20is%3Aopen%20type%3AEpic)               | Tracks a collection of related issues or tasks under a larger initiative |

> [!TIP]
> Not sure what type to use? Pick the **Task** type. 

## 🔀 Pull requests

Pull requests (PRs) are used to propose changes to the project. Each PR should
is reviewed to ensure quality, maintainability and alignment with project goals
before merging.

After a PR is submitted it is automatically checked by the project CI/CD
pipeline. This also being the
[code review](/CONTRIBUTING.md#code-review)
process where maintainers and the community asses the proposed changes, provide
feedback and request modifications as needed. Once the PR is approved and all
checks pass, maintainers will merge it into the repository.

> [!TIP]
> Read the [CONTRIBUTING](/CONTRIBUTING.md) guide for PR tips/guidelines 

## 🎯 Milestones

This project uses milestones to track work for upcoming releases. Milestones
define when an issue, pull request, and/or roadmap item is to be completed.
Issues are the what, milestones are the when. Roadmap items can move between
milestones depending on the remaining development and testing required to
release a change.

For more information on the release process please refer to [RELEASES](/RELEASES.md).

### Project roadmap

Roadmaps frequently get out-of-date quickly when they are part of the repository
content. Moby uses a **GitHub Project** to collect and show the current roadmap.

## 🔖 Tags

All releases of moby will have a corresponding GPG signed tag following the
version format defined in [RELEASES].

## 🌿 Branches

`master` serves as the development branch for future releases of the project.
All changes should be made to the `master` branch, and changes to release
branches should only be made in the form of cherry-picked commits, if possible.
The sponsoring maintainers of a release branch serve as the primary point of
contact, and are available to provide guidance on contributing changes to their
respective branches.

### Maintenance status

The maintenance status of a branch defines the level of support provided by
maintainers and contributors. This status helps set expectations for
contributors regarding which changes may be accepted into a given branch and
clarifies the scope of security and bug fixes.

- __*Active*__: The release is a stable branch which is currently supported and
  accepting patches.
- __*Extended*__: The release branch is only accepting critical backports and
  security patches.
- __*Security*__: The release branch is only accepting security patches.
- __*End of Life*__: The release branch is no longer supported and no new
  patches will be accepted.

> [!NOTE] 
> The Moby Project provides source code releases. Binary distributions
> are available from multiple contributing parties, and known distributions can
> be discovered in [PACKAGING.md](PACKAGING.md).


| Release | Maintenance status | End of maintenance        | Sponsors                 | Distributors                      |
| ------- | ------------------ | ------------------------- | ------------------------ | --------------------------------- |
| 23.0    | Security           | [2025-05-19][mcr23-maint] | @corhere                 | [Mirantis][mcr]                   |
| 24.0    | End of life        | --                        | --                       | --                                |
| 25.0    | Extended           | TBD                       | @corhere, @austinvazquez | [Amazon][al2023], [Mirantis][mcr] |
| 26.0    | End of life        | --                        | --                       | --                                |
| 27.x    | Extended           | TBD                       | @cpuguy83                | [Microsoft][msft]                 |
| 28.0    | Active             | --                        | @moby/committers         | [Docker][docker]                  |

[al2023]: https://docs.aws.amazon.com/linux/
[docker]: https://docker.com
[mcr23-maint]: https://docs.mirantis.com/mcr/23.0/compat-matrix/maintenance-lifecycle.html
[mcr]: https://www.mirantis.com/software/mirantis-container-runtime/
[msft]: https://microsoft.com

> [!NOTE]
> All releases prior to `v23.0.0` are no longer maintained

### Backports

Backports in moby are community driven. As maintainers, we'll try to
ensure that sensible bugfixes make it into _active_ release, but our main focus
will be features for the next _minor_ or _major_ release. For the most part,
this process is straightforward, and we are here to help make it as smooth as
possible.

If there are important fixes that need to be backported, please let us know in
one of three ways:

1. Open an issue.
2. Open a PR with cherry-picked change from `master`.
3. Open a PR with a ported fix.

__If you are reporting a security issue:__

Please follow the instructions in [SECURITY](/SECURITY.md)

Remember that backported PRs must follow the versioning guidelines from this document.

Any release that is not "end of life" can accept backports. Opening a backport PR is
fairly straightforward. The steps differ depending on whether you are pulling
a fix from `master` or need to draft a new commit specific to a particular
branch.

## 📊 Workflow and tracking

Issues and pull requests each follow the same basic workflow, though some changes may be expedited (e.g.,
security vulnerabilities or critical regressions).

## 🚑 Triage

Triage provides an important way to contribute to an open-source project. Triage helps ensure work items
are resolved quickly by:
  
- Ensuring that issues and pull requests are clearly described.
- Providing contributors with the necessary context before they accept or commit to a change.
- Preventing duplicate submissions and unnecessary rework.
- Keeping discussions focused and avoiding fragmentation

#### Content review

Before advancing the triage process, ensure the issue/PR contains all necessary
information to be properly understood and assessed. The required information may
vary by type of work -- ensure that the issue/PR contains (or links to) all relevant context.

- **Exercising Judgment**: Use your best judgment to assess the issue/PR
  description’s completeness.
- **Communicating Needs**: If the information provided is insufficient, kindly
  request additional details from the author. Explain that this information is
  crucial for clarity and resolution of the item, and apply the
  `needs/more-information` label to indicate a response from the author is
  required.

#### Classification and labeling

Issues and pull requests will typically have multiple [labels](#labels) to help
communicate key information. Issues should also have a specific
[type](#types-of-issues). At a minimum, a properly classified issue or pull
request should have:

- (Required) One or more [`area/*`](https://github.com/moby/moby/labels?q=area) labels
- (Required) An [issue type](#types-of-issues) or [`kind/*`](https://github.com/moby/moby/labels?q=kind) label for pull requests
  
Additional labels can provide more clarity:

- Zero or more [`needs/*`](https://github.com/moby/moby/labels?q=needs) labels to indicate missing items
- Zero or more [`impact/*`](https://github.com/moby/moby/labels?q=impact) labels to identify major areas impacted by the change
- One [`exp/*`](https://github.com/moby/moby/labels?q=exp) label to estimate the difficulty of the work
- A [`priority/*`](https://github.com/moby/moby/labels?q=priority) label (required for `Bug` types)

> [!TIP] 
> Most labels should have a description, read this for for information or
> post a [discussion in Q&A](https://github.com/moby/moby/discussions/categories/q-a)

### Backlog

Items that have gone through triage but don't have an assigned milestone or
GitHub project are considered to be part of the backlog. The current backlog can
be viewed by looking at open issues or pull requests that do not have the
`needs/triage` label and have no milestone or project assigned to them.

## 📌 Projects

GitHub [projects](https://github.com/moby/moby/projects?query=is%3Aopen) are
used to support [project](#-workflow-and-tracking) operations. These boards can
be used to track the progress of changes, expected features in a release and
insight into the Moby roadmap. Read the project documentation for more
information about the views and fields being used.

## 💬 Discussions

[GitHub discussions](https://github.com/moby/moby/discussions) serve a community
forum for this project. Community members can ask questions, share updates and
have open-ended discussions on project-related topics.

Sometimes, an issue or pull request may not be the appropriate medium for what
is essentially a discussion. In such cases, the issue or PR will either be
converted to a discussion or a new discussion will be created.

If you believe this conversion was made in error, please express your concerns
in the new discussion thread. If necessary, a reversal to the original issue or
PR format can be facilitated.

## ⚙️ Workflow automation

To help expedite common operations, avoid errors and reduce toil some workflow
automation is used by the project. This can include:

- Stale issue or pull request processing
- Auto-labeling actions
- Auto-response actions
- Label carry over from issue to pull request

### Exempting an issue/PR from stale bot processing

The stale item handling is configured in the [repository](link-to-config-file).
To exempt an issue or PR from stale processing you can:

- Add the item to a milestone
- Add the `state/frozen` label to the item

## 🌎 Public APIs

TODO