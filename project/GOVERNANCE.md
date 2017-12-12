# Moby project governance

Moby projects are governed by the [Moby Technical Steering Committee (TSC)](https://github.com/moby/tsc).
See the Moby TSC [charter](https://github.com/moby/tsc/blob/master/README.md) for
further information on the role of the TSC and procedures for escalation
of technical issues or concerns.

Contact [any Moby TSC member](https://github.com/moby/tsc/blob/master/MEMBERS.md) with your questions/concerns about the governance or a specific technical 
issue that you feel requires escalation.

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

### Adding maintainers

Maintainers are first and foremost contributors who have shown their
commitment to the long term success of a project. Contributors who want to
become maintainers first demonstrate commitment to the project by contributing
code, reviewing others' work, and triaging issues on a regular basis for at
least three months.

The contributions alone don't make you a maintainer. You need to earn the
trust of the current maintainers and other project contributors, that your
decisions and actions are in the best interest of the project.

Periodically, the existing maintainers curate a list of contributors who have
shown regular activity on the project over the prior months. From this
list, maintainer candidates are selected and proposed on the maintainers
mailing list.

After a candidate is announced on the maintainers mailing list, the
existing maintainers discuss the candidate over the next 5 business days,
provide feedback, and vote. At least 66% of the current maintainers must
vote in the affirmative.

If a candidate is approved, a maintainer contacts the candidate to
invite them to open a pull request that adds the contributor to
the MAINTAINERS file. The candidate becomes a maintainer once the pull
request is merged.

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
repository. Weekly meetings are organized to are organized to synchronously
discuss tricky PRs, as well as design and architecture decisions.. When
technical agreement cannot be reached among the maintainers of the project,
escalation or concerns can be raised by opening an issue to be handled
by the [Moby Technical Steering Committee](https://github.com/moby/tsc).
