# Moby project governance

Moby is the open-source project used to build Docker, jointly maintained by Docker
and the community. As a [community project](https://www.docker.com/community/open-source/),
we abide by a [Code of Conduct](https://github.com/docker/code-of-conduct)
and define a governance which ensures the project is community driven and open to
anyone to contribute.

Contact moby@docker.com with any questions/concerns about the enforcement of the
Code of Conduct.

## Project maintainers

The current maintainers of the moby/moby repository are listed in the
[MAINTAINERS](/MAINTAINERS) file.

There are different types of maintainers, with different responsibilities, but
all maintainers have 3 things in common:

 1. They share responsibility in the project's success.
 2. They have made a long-term, recurring time investment to improve the project.
 3. They spend that time doing whatever needs to be done, not necessarily what is the most interesting or fun.

Maintainers are often under-appreciated, because their work is less visible.
It's easy to recognize a really cool and technically advanced feature. It's harder
to appreciate the absence of bugs, the slow but steady improvement in stability,
or the reliability of a release process. But those things distinguish a good
project from a great one.

## Reviewers

A reviewer is a maintainer within the project.
They share in reviewing issues and pull requests and their LGTM counts towards the
required LGTM count to merge a code change into the project.

Reviewers are part of the organization but do not have write access to the project.
Becoming a reviewer is a core aspect in the journey to becoming a committer.

## Committers

A committer is a maintainer who is responsible for the overall quality and
stewardship of the project. They share the same reviewing responsibilities as
reviewers, but are also responsible for upholding the project bylaws as well as
participating in project level votes.

Committers are part of the organization with write access to the project.
Committers are expected to remain actively involved in the project and
participate in voting and discussing of proposed project level changes.


## Adding maintainers

Maintainers are first and foremost contributors that have shown they are
committed to the long term success of a project. Contributors wanting to become
maintainers are expected to be deeply involved in contributing code, pull
request review, and triage of issues in the project for more than three months.

Just contributing does not make you a maintainer, it is about building trust
with the current maintainers of the project and being a person that they can
depend on and trust to make decisions in the best interest of the project.

Periodically, the existing maintainers curate a list of contributors that have
shown regular activity on the project over the prior months. From this list,
maintainer candidates are selected and proposed in the maintainers forum.

After a candidate has been informally proposed in the maintainers forum, the
existing maintainers are given seven days to discuss the candidate, raise
objections and show their support. Formal voting takes place on a pull request
that adds the contributor to the MAINTAINERS file. Candidates must be approved
by 2/3 of the current committers by adding their approval or LGTM to the pull
request. The reviewer role has the same process but only requires 1/3 of current
committers.

If a candidate is approved, they will be invited to add their own LGTM or
approval to the pull request to acknowledge their agreement. A committer will
verify the numbers of votes that have been received and the allotted seven days
have passed, then merge the pull request and invite the contributor to the
organization.

### Removing maintainers

Maintainers can be removed from the project, either at their own request
or due to [project inactivity](#inactive-maintainer-policy).

#### How to step down

Life priorities, interests, and passions can change. If you're a maintainer but
feel you must remove yourself from the list, inform other maintainers that you
intend to step down, and if possible, help find someone to pick up your work.
At the very least, ensure your work can be continued where you left off.

After you've informed other maintainers, create a pull request to remove
yourself from the MAINTAINERS file.

#### Inactive maintainer policy

An existing maintainer can be removed if they do not show significant activity
on the project. Periodically, the maintainers review the list of maintainers
and their activity over the last three months.

If a maintainer has shown insufficient activity over this period, a project
representative will contact the maintainer to ask if they want to continue
being a maintainer. If the maintainer decides to step down as a maintainer,
they open a pull request to be removed from the MAINTAINERS file.

If the maintainer wants to continue in this role, but is unable to perform the
required duties, they can be removed with a vote by at least 66% of the current
maintainers. The maintainer under discussion will not be allowed to vote. An
e-mail is sent to the mailing list, inviting maintainers of the project to
vote. The voting period is five business days. Issues related to a maintainer's
performance should be discussed with them among the other maintainers so that
they are not surprised by a pull request removing them. This discussion should
be handled objectively with no ad hominem attacks.

## Project decision making

Short answer: **Everything is a pull request**.

The Moby core engine project is an open-source project with an open design
philosophy. This means that the repository is the source of truth for **every**
aspect of the project, including its philosophy, design, road map, and APIs.
*If it's part of the project, it's in the repo. If it's in the repo, it's part
of the project.*

As a result, each decision can be expressed as a change to the repository. An
implementation change is expressed as a change to the source code. An API
change is a change to the API specification. A philosophy change is a change
to the philosophy manifesto, and so on.

All decisions affecting the moby/moby repository, both big and small, follow
the same steps:

 * **Step 1**: Open a pull request. Anyone can do this.

 * **Step 2**: Discuss the pull request. Anyone can do this.

 * **Step 3**: Maintainers merge, close or reject the pull request.

Pull requests are reviewed by the current maintainers of the moby/moby
repository. Weekly meetings are organized to synchronously
discuss tricky PRs, as well as design and architecture decisions.

### Conflict Resolution

If you have a technical dispute that you feel has reached an impasse with a
subset of the community, any contributor may open an issue, specifically
calling for a resolution vote of the current committers to resolve the
dispute. A resolution vote must be approved by 2/3 of the current
committers.
