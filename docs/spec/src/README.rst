#########################
 Behavior specifications
#########################

Each spec here specifies a part of the Moby daemon as it is observable from outside the daemon. A
spec states:

- what that part is expected to do
- which of that behavior is promised, which is only today's behavior, and which is known to be wrong
- the vantage point from which to observe it
- what the verdict of an expectation depends on

This document gives the model that every spec follows: the items of a spec, how an expectation is
read, and how the items link to each other. It covers these documents:

.. list-table::
    :header-rows: 1

    - - Document
      - Scope
    - - :doc:`principles.md <principles>`
      - The principles that the maintainers hold the daemon to, in every spec
    - - :doc:`id-style-guide.md <id-style-guide>`
      - How the IDs and titles of items are formed

A spec describes behavior only. It should change when the behavior of the daemon changes. It should
also change when a regression reveals an expectation that, until then, was only an assumption.

.. _readme-who-the-specs-serve:

*********************
 Who the specs serve
*********************

The specs serve two kinds of test. Functional tests serve the maintainer's story: *"as a Moby
maintainer, catch regressions in code changes."* Acceptance tests serve the packager's story: *"as a
packager, verify a candidate release works on every platform in my support matrix."* :ref:`Depends
on <readme-depends-on>` tells which expectations have a verdict that can differ between OS releases.

The daemon can have facilities built in for testing, such as a failpoint or a timer override. A
packaged build can ship such a facility, enabled only by an environment variable whose name starts
with :env:`DOCKER_TEST_`, as the daemon's existing networking test hooks are. A duration that an
expectation or its subsystem's README states, such as a convergence ceiling or the 12 hours between
two key rotations, is that of a daemon without a timer override. A run in which an override changes
the duration does not decide the expectation's verdict. A test facility can still bring about the
state that an expectation's *Applies to* paragraph gives.

.. _readme-expectations:

**************
 Expectations
**************

A spec is a series of sections. Each section has a list of expectations, the items of the type
``expectation``. An expectation has an ID and a title, the vantage point from which to observe it,
what its verdict depends on, the platforms where its case occurs, links to other items, and a body.
The body gives the state that the expectation applies to, in an *Applies to* paragraph, then the
statement of the action and its outcome. The verdict of an expectation is whether its statement
holds in a given setup. The links of an expectation decide its category: a defect, a contract, a
provisional expectation or a detector (:ref:`Categories <readme-categories>`). A spec can also say
what it does not cover, and what it deliberately does not assert.

Every sentence of a statement is falsifiable: it says what the daemon does, so that an observation
can contradict it. A statement does not say what can or may happen, or what is not guaranteed. This
holds for a defect too: a defect states today's behavior.

An expectation that says that the daemon leaves something unchanged, or does not do something, names
a case in which the opposite would show, where the baseline state cannot show it: for example, a
device that exists beforehand with a value other than the default. An expectation that says that the
daemon sets something to a value, where the baseline state can already have that value, names a case
in which it has another value before the action.

The *Applies to* paragraph at the start of an expectation's body gives the state before the
expectation's action. Examples are objects that exist, daemon settings, the platform, and what
happened to an object before the action. It never names the action itself. A request, a command, an
update or a restart that the expectation is about goes in the statement. An expectation without an
*Applies to* paragraph applies whenever its section does. An *Applies to* paragraph states the scope
of its own expectation. It does not point at the expectations that cover the cases outside that
scope. Two expectations that make the same promise in different scopes refine the same requirement.

An expectation states the outcome of an action at the same level of abstraction as the action. The
outcome of an Engine API request is the API's response: its status and body. The outcome of a
:cli:`docker` command is what the command prints and its exit status. An HTTP status is not visible
at the level of a :cli:`docker` command. An expectation about a :cli:`docker` command names ``cli``
among what it depends on. It usually has a partner expectation about the API request that the
command makes, which it links with ``cli_for``.

An outcome covers only the direct response to the action. A side effect is not part of the outcome.
Examples of side effects are a network created or removed, traffic on the wire, and an entry in the
daemon's log. A spec states a side effect where it can be observed, whatever the action was. An
expectation that an action succeeds also states the side effect that shows the action took effect. A
daemon that reports success and does nothing fails such an expectation.

Unless it names a range, an expectation about an Engine API request holds at every API version that
the daemon accepts by default. These versions go from the daemon's default minimum version to the
current version.

An expectation names a daemon setting by its :file:`daemon.json` key, such as
:daemon-json:`firewall-backend` or :daemon-json:`iptables`. The matching :program:`dockerd` flag,
such as :dockerd-flag:`--firewall-backend`, sets the same setting. An expectation holds whichever of
the two sets it. An expectation that names a :program:`dockerd` flag, such as :dockerd-flag:`--mtu`,
holds only when that flag sets the setting. Any other flag that an expectation names, such as
:cli-flag:`--dns`, is a flag of a :cli:`docker` command.

Unless its own text, its section or its spec says otherwise, an expectation holds on a node in its
baseline state. For a daemon installed from a package or an installer, the baseline is a clean
install of a supported OS, a fresh install of the package, and the setup that the product's
installation documentation prescribes. The package and that documentation are part of the system
under test. For a daemon that is not installed from a package, such as one built in CI, the baseline
is a host that meets the host requirements of these specs, with no other configuration. The host
requirements are the daemon's own requirements, whatever the distribution.

The state in an expectation's *Applies to* paragraph describes changes from the baseline. An
expectation holds with each daemon setting that it does not name as the baseline leaves it: as the
package installs it, or at its default for a daemon that is not installed from a package. It holds
at another value only where the expectation or its section says so. A subsystem's README can make an
exception for a setting.

An expectation does not cover the case that a defect narrowing it states. The defect ``narrows`` the
expectation, and says what happens instead. On the narrowed expectation, the link reads *Does not
cover*, followed by the defect's title and ID. An expectation about a request covers only requests
that no expectation other than a defect refuses because of a field or option that this expectation
does not name, or because of a state that this expectation does not name of an object that the
request names, directly or through the container that it names. Which refusal a request with several
such fields or states gets is not stated unless an expectation names that combination. A defect
narrows the expectations that it contradicts only through its links, and through a limit that its
subsystem's README names. No other expectation limits the scope of an expectation: it holds in every
case that its own text, its section, its spec, the rules of this document and the conventions of its
subsystem's README leave it.

The release under test is *N*. The previous release, *N−1*, is each of two releases: the previous
patch release of *N*, and the latest patch release of the previous minor release line. An
expectation about *N−1* holds for each of them. A build from a release branch counts as the next
patch release of that branch. A build from the main branch, such as a pull request, counts as the
next minor release, so its *N−1* is the latest patch release of the latest minor release line. For
an x.y.0 release, the two releases are the same. An expectation about *N−1* holds whether *N−1* runs
from its package or from its released binaries. Apart from what the package's upgrade path decides,
an expectation that names ``upgrade`` also holds when the daemon's binaries are replaced in place of
a package upgrade. An expectation about *N−1* asserts the behavior of *N*. When a known bug in an
*N−1* release breaks such an expectation, the expectation does not cover the combination that the
bug breaks, until *N−1* is a release without the bug. A bug is known when *N* has its fix and that
*N−1* release does not.

.. _readme-general-timing:

Timing
======

A request that a node's daemon carries out itself, such as, in the networking specs, a request about
a container or its endpoints, whatever the scope of the network, or about a network that is not
Swarm-scoped, takes effect on that node once the request responds. The exception is an effect that
depends on state from other nodes or from the Swarm's managers, such as the records of other nodes'
endpoints or the removal of a node's last attachment to a Swarm-scoped network, which its spec's
timing rules bound by a convergence ceiling. An expectation that links a ceiling for the effects of
the same request on other nodes, or for its other cases, holds on the requesting node once the
request responds. A request that is refused has no effect to wait for. An expectation about what a
refused request leaves unchanged holds once the request responds. An expectation about what holds
after the daemon starts holds once the daemon's API responds, without a ceiling, on a node in a
Swarm too, unless the expectation names a later point.

An expectation that says that a request responds after a number of seconds, or that the daemon waits
that long before it responds, holds when the response comes no earlier than that many seconds after
the request is sent, and no more than 5 seconds later.

An expectation, or a sentence of an expectation, that says that something does not happen, without
an action, holds once an expectation or a sentence that says that something does happen in the same
setup holds. An expectation that links a convergence ceiling is timed instead by the rules that its
spec gives for ceilings.

An effect that converges over time, such as an effect in a Swarm, holds within a convergence ceiling
that its spec gives.

.. _readme-categories:

Categories
==========

Every expectation has a category. Nobody writes it: the build computes it from the expectation's own
links, by the first line of this table that fits.

.. needreport::
    :types:
    :template: templates/categories.rst.j2

An expectation is a contract when it ``refines`` a requirement or a principle (:ref:`Requirements,
principles and rationales <readme-requirements-principles-and-rationales>`). The behavior is
promised: a source says so, the maintainers hold the daemon to it, or users observably rely on it. A
change to a contract needs a major release, or an option that restores the behavior, unless a
principle that it refines asks for more. That a maintainer means to change a behavior later does not
make its expectation less than a contract. Every sentence of a contract is covered by something that
the expectation refines. A sentence that nothing covers moves to a detector of its own.

An expectation is a defect when it ``violates`` a requirement or a principle. A defect states what
the daemon does today in a case where that behavior contradicts a promise. The specs do not decide
in advance what a change to that behavior is. Whether a change that makes a defect false is a fix or
a break for the users who met the behavior is weighed when the change is made. A defect ``narrows``
each expectation that it contradicts in its case, and ``concerns`` each expectation whose case it is
about without contradicting it. It links at least one expectation in one of these two ways. A defect
sits in a section that holds at least one of the expectations that it narrows or concerns, the one
that holds most of them when several do, and has an ID of the same form as theirs. Within that
section, it sits directly before or directly after one of those expectations, one of that
expectation's exact or CLI forms, or one of the other defects next to them. An expectation that pins
the defect can follow it. The README of a subsystem's specs can name a subsection of that section
for the defects of one kind of case, and such a defect sits in that subsection.

An expectation is provisional when it is ``provisional_on`` a requirement whose own source says that
the behavior may change. It is promised with that caveat, and a change to it is weighed against the
caveat. A provisional expectation narrows nothing. An expectation that has a stable basis as well
refines that basis and is a contract. When the source drops its caveat, the provisional expectation
becomes a contract.

An expectation with none of these links is a detector. It states today's behavior so that the
behavior does not change by accident. Users are not known to rely on it, and no source promises it.
A detector says what it is for in one of three ways:

- It is the ``exact_form_of`` another expectation: it states the exact form of something that the
  other expectation states more loosely. An example is the particular HTTP status of a refusal,
  where the contract promises only that the request is refused with a 4xx status.
- It ``pins`` a host requirement or another expectation whose behavior it asserts.
- It links a ``rationale`` (:ref:`Requirements, principles and rationales
  <readme-requirements-principles-and-rationales>`).

The category reads only the expectation's own links. An expectation that refines a requirement is a
contract even where that requirement conflicts with a principle. The requirement records the
conflict, and the expectation does not repeat it.

An expectation is about one behavior. Where a part of a behavior is promised and another part of it
contradicts a promise, the parts are two expectations: a contract for the first and a defect for the
second. An example is a request with a setting that the daemon accepts and then ignores. The
contract states that the request is accepted. The defect states that the setting has no effect.

A behavior can have both a contract and a detector. In the networking specs, the inner MTU of a
container on an overlay network is the model case: a contract states it as a property of the path,
and a detector states the pinned value.

.. _readme-vantage-points:

Vantage points
==============

A black-box expectation means something only from a stated vantage point. Some expectations hold
from one vantage point and not from another. Every expectation states its vantage point in
``observed_from``, a defect too. The README of a subsystem's specs lists the vantage points that
they use.

A contract about the daemon log refers to the entry with the text that the detector that is its
exact form gives. A log contract without an exact form of its own, such as an expectation about a
failure that repeats or clears, refers to the entries that the exact form of its section's contract
about the same failure gives. A deliberate change to that text makes both expectations false. An
expectation that says that the log does not report something holds when no entry with that text
appears after the last action or change that the expectation states before it, or since the daemon
started when the expectation states none. Where such an expectation gives no text, as when it says
that a log has no error or no entry about a failure, that it does not report a failure, or that only
another log reports it, an entry counts when it is at the warning level or above, contains the name
or ID of an object that the failure is about, such as a network, a service, a task or a container,
or the address of a node or an endpoint that the failure is about, and reports that failure, not
another event of the same object. Where the expectation says that the log has no error, only an
entry at the error level or above counts. An expectation that states its own scope or level, such as
one that names the log level, uses that instead. An expectation without an exact form that says only
that the log contains an error holds when an entry reports the error that the expectation describes.
The daemon log is everything that the daemon process writes to its standard output and standard
error, as a service manager such as systemd collects it. For a daemon that does not start, it
includes an error that the daemon prints before its logging is set up, which has no log level. Such
an entry reports the error when it is about the failure that the expectation describes; it need not
name a setting or have a level unless the expectation says so or has an exact form.

.. _readme-depends-on:

Depends on
==========

An expectation names, in ``depends_on``, the facilities that its verdict depends on, or ``none``. An
expectation names ``os-linux`` or ``os-windows`` for each platform in its ``platforms`` whose
operating system can decide its verdict. It names ``none`` only when nothing outside the daemon
build can decide its verdict, and then names nothing else.

.. needreport::
    :types:
    :template: templates/depends-on.rst.j2

What a value means for an expectation's scope:

    An expectation that names a facility holds on every OS release in a packager's support matrix
    that runs a platform in its ``platforms``, whatever its category, with the facility that the OS
    release supplies. Its verdict can differ from one OS release to another. The verdict of an
    expectation that names ``none`` does not differ between OS releases that run the same daemon
    build.

An expectation names a facility if the facility decides the verdict when the real facility is in
use. An expectation whose state fixes how a facility behaves, such as a kernel that does not offer
XFRM, names ``none`` when the daemon alone decides the verdict in that state.

.. _readme-platforms:

Platforms
=========

An expectation holds on the nodes of the platforms that its ``platforms`` lists. A section built on
a Linux-only mechanism says so in its introduction, and its expectations list only ``linux``. An
expectation that depends on a Linux-only daemon setting, such as the userland proxy or the firewall
backend, lists only ``linux``.

An expectation's ``platforms`` lists the platforms where its case occurs. The platform is that of
the node whose behavior the expectation states.

.. needreport::
    :types:
    :template: templates/platforms.rst.j2

A platform's different behavior is its own expectation:

- When the whole case of an expectation differs on one platform, the expectation lists only the
  other platform. A separate expectation with the first platform states what the daemon does there.
  That expectation is a defect where its behavior violates a requirement or a principle. Otherwise
  it is a contract or a detector, as its links decide.
- There is one such expectation for each behavior, not one for each expectation that it replaces.
  When it is a defect, it ``concerns`` every expectation that it replaces on its platform.
  Expectations come in pairs, one for each platform, only where the platform's behavior differs from
  one expectation to another.
- An expectation that a platform's behavior contradicts only in some case keeps both platforms. The
  defect that states the behavior ``narrows`` it.
- An expectation is narrowed to one platform only on evidence that the other platform behaves
  differently: a published source, a run, or code that fully decides the behavior.
- When the same case has the same outcome on another platform, the expectation lists that platform
  too. There is no second expectation.
- Where the daemon's own code fully decides the behavior on a platform, an expectation states it.
  Where a component outside the daemon decides it, such as the Host Networking Service or the
  Virtual Filtering Platform, and the behavior is known only from reading code, no expectation
  states it.
- An expectation that gives a closed list or set gives the whole of it, on every platform that the
  expectation lists.

.. _readme-items:

*******
 Items
*******

Each expectation, host requirement, convergence ceiling, requirement, principle and rationale in
these specs is a `Sphinx-Needs <https://sphinx-needs.readthedocs.io/>`__ item with an ID, such as
``BR-ISO-NO-ICC``, that tests and other items cite.

.. needreport::
    :types:
    :template: templates/types.rst.j2

An ID never changes once it is merged. When a merged item is removed, its ID is listed in
``retired-ids.txt`` and never used again. An expectation, a host requirement or a convergence
ceiling has an ID of the form ``<SPEC>-<AREA>-<SLUG>``. A requirement has an ID of the form
``REQ-<SPEC>-<SLUG>``, and a rationale one of the form ``RAT-<SPEC>-<SLUG>``. A principle holds
across the specs, so its ID has no spec code: ``PRIN-<SLUG>``. An ID does not show the category of
an expectation. The category follows the expectation's links, which can change, and the ID cannot.
:doc:`id-style-guide` says how IDs and titles are formed.

An item's fields hold single values or lists of names, so that they can be filtered and checked. An
expectation in a section that lists the options of a driver, such as the options of the overlay
driver, names in ``key`` the options that it is about. Its body holds the prose. Inline code in a
body has a role that says what it names, such as ``:api-op:`` for an Engine API operation,
``:dockerd-flag:`` for a flag of :program:`dockerd`, ``:daemon-json:`` for a daemon setting,
``:kconfig:`` for a kernel option or ``:ipt-chain:`` for an iptables chain. The build checks the
text of several roles against the repository: an API operation against
:repo-path:`api/openapi.yaml`, a kernel option against the host requirements, a chain against the
reference rulesets, a driver option and a :program:`dockerd` flag against the daemon's code.

A body refers to another item by its ID, with the ``:need:`` role, and to a section by the section's
label, with the ``:ref:`` role, which shows the section's title. Headings have no numbers, and
nothing in the specs cites a section by number. A reference to a section of another document names
that document too, as in ``:ref:`swarm-lifecycle-convergence` in :doc:`network/swarm-lifecycle.md
<network/swarm-lifecycle>```. Where the sentence itself says what the section holds, the name of the
document can carry the link to the section instead. The label of a section is the name of its
document, then the words of its title, as in ``swarm-lifecycle-failure-reporting``. Where two
documents share a name, as the two READMEs do, and a section of each has the same title, the label
of one of the two adds a word that tells them apart, as ``readme-general-timing`` and
``readme-network-platforms`` do. Where two sections of one document share a title, the label of each
adds the words of its parent section's title before its own.

The sources of the specs are the reStructuredText files in ``src/``. The Markdown files in this
directory and its subdirectories are generated from them by ``make md``. Each starts with a comment
that says so. ``make fmt`` formats the sources, and ``make validate`` checks that they build without
warnings, that they are formatted, and that the Markdown files match them. CI runs ``make
validate``.

.. _readme-requirements-principles-and-rationales:

*****************************************
 Requirements, principles and rationales
*****************************************

The expectations rest on three types of item. A requirement or rationale is at the start of a
document of the spec that its ID names: the first of them that links or names it. The principles
hold for every spec, and are in :doc:`principles.md <principles>`. They say why a behavior is
promised, or why a change to it should be deliberate.

Requirements
============

A requirement is a promise that a cited source makes. The source is the documentation, a revision of
it that is no longer current, the Engine API reference, an issue or pull request in which the
maintainers agreed that a behavior must hold (in a comment, in a merged change's commit message or
code comment, or by triaging as valid an issue that expects the behavior), or behavior that users
observably rely on. Its ``source_kind`` says which:

.. needreport::
    :types:
    :template: templates/source-kinds.rst.j2

A statement that only a revision of the documentation that is no longer current makes is a promise
when a reasonable user could have built on it and a change in a minor release would break what they
built. A requirement states the promise in the specs' own words and cites its sources. Its statement
says what its sources promise, no more and no less, read by their plain meaning. A sentence of a
source that only warns that a request can fail, explains how the daemon brings about a promised
outcome, tells operators what to configure, or recommends or warns against a configuration, is not
part of the promise. Expectations state the behavior that a warning is about, and host requirements
state what a node must provide. Behavior that contradicts a sentence with a source is a defect. It
is not a reason to reword the requirement. Where two current sources disagree, the one that states a
later change made on purpose, such as a release note or the commit message of a merged change, gives
the promise, and the requirement cites both. A requirement whose wording is general does not reach
beyond the feature that its source covers. Some expectation refines every clause of a requirement's
statement.

Reliance is a source. When the reason to keep a behavior is that users rely on it, the specs have an
``observable`` requirement that names who relies on the behavior and how, and the expectations about
that behavior refine or violate it.

A requirement can ``refine`` the principles that it serves. It can also ``violate`` a principle:

- A requirement whose own source says that the behavior may change violates ``PRIN-STABLE``. An
  example is a page of the documentation that says that the options, behavior and implementation of
  a feature may all change in future releases. An expectation is ``provisional_on`` such a
  requirement, and never ``refines`` it.
- A requirement that violates any other principle is a binding promise that is known to conflict
  with that principle. Its expectations stay contracts, and only a major release, or an option that
  restores the behavior, changes them. The requirement's *Violated by* list collects what a major
  release should reconsider. An example in the networking specs is the documented default that
  attaches a container to the default bridge network, which grants access that the user did not ask
  for.

Principles
==========

A principle is a property that the maintainers hold the daemon to across the specs. It has no
source. A principle ``refines`` broader principles, and only principles. A principle that refines
none is a root. There are six roots:

.. list-table::
    :header-rows: 1

    - - Root
      - What it says
    - - ``PRIN-USER-INFORMED``
      - The daemon keeps the user informed. It reports a problem with a request at the earliest
        point it can, in terms that the user can act on.
    - - ``PRIN-FAITHFUL``
      - The daemon does what a request asks, or nothing.
    - - ``PRIN-OWNS-ONLY-ITS-OWN``
      - The daemon manages only what it creates. It removes what it creates, and leaves alone what
        it did not create.
    - - ``PRIN-NODE-LOCAL``
      - A network of local scope is a resource of the node that has it, and only the objects on that
        node use it.
    - - ``PRIN-LEAST-ACCESS``
      - The daemon grants no access that the user did not ask for.
    - - ``PRIN-STABLE``
      - What a release does at the daemon's interfaces, later minor releases keep doing. A change
        needs a major release, or an option that restores the behavior.

The table gives the gist of each root. The items in :doc:`principles.md <principles>` give their
statements. Principles, requirements and expectations can refine a principle. An expectation refines
every principle and requirement whose statement covers it directly, whichever spec the requirement
belongs to.

An anchor is a name or place in host state that a spec names for others to build on, such as a chain
that operators add rules to. A spec lists its anchors. Nothing else that the daemon creates on a
host is an anchor. ``PRIN-STABLE`` covers such state only where users observably rely on it, as an
``observable`` requirement records.

Rationales
==========

A rationale says why a change to a detector's behavior should be deliberate. It says what change the
detector notices, and what an accidental change would disturb, such as operators' tooling,
debugging, or consistency with a sibling behavior. A rationale never says that users rely on the
behavior. When that is the honest reason, the behavior has an ``observable`` requirement, and its
expectation refines that requirement and is a contract. A detector for which neither a rationale nor
a requirement can be written has no place in a spec.

A detector that is not an exact form and pins nothing links one or more rationales. Several
detectors can share one rationale. Every rationale explains at least one expectation.

Links
=====

Items link to each other, and the schema in ``conf.py`` checks the links:

- An expectation ``refines`` every requirement and principle whose statement covers it directly.
- An expectation is ``provisional_on`` a requirement that violates ``PRIN-STABLE``. An expectation
  ``refines`` no such requirement.
- An expectation ``violates`` every requirement whose statement a sentence of the expectation
  contradicts directly. A requirement that is reached only by a reading of the expectation is not a
  target. The rules for the target are these:

  - A defect that negates, in its case, a contract that it narrows violates the requirement whose
    clause covers that case, among those that the contract refines, even when the requirement is
    worded in other terms. The contract is how the specs read that clause. A requirement that the
    contract refines is not a target when the defect's case falls under none of its clauses.
  - For each of its sentences, a defect violates the most specific target that the sentence
    contradicts directly. Where a sentence contradicts no requirement directly, the first choice is
    a requirement whose statement can be extended to what its own sources say, and the second is a
    new requirement with a source that can be cited. Only when there is neither is that sentence's
    target a principle.
  - A purpose clause of a requirement, such as "so that …", is never contradicted directly.
  - A violation does not pass through an exact form. The defect that states the outcome carries the
    link.
  - A sentence is added to a defect to make a contradiction direct only when the defect has no
    direct target without it.
  - Behavior that contradicts no requirement and no principle is not a defect. Its expectation is a
    detector with a rationale.

- A defect ``narrows`` the expectations that it contradicts in some case, and ``concerns`` the
  expectations whose case it is about without contradicting them.
- An expectation is the ``exact_form_of`` at most one expectation. An expectation ``pins`` a host
  requirement or another expectation whose behavior it asserts, as a detector does to say what it is
  for, and as a contract or a defect does when it states what the daemon does on a host without a
  requirement. A defect that is an exact form, or that pins another expectation, carries its own
  violations: the link only says which looser statement it details or which behavior it relies on. A
  detector links the ``rationale`` items that say why a change to it should be deliberate.
- An expectation ``needs`` the host requirements that its verdict relies on, is ``within`` the
  convergence ceilings that bound it, and ``assumes`` the driver properties it relies on.
- An expectation about a :cli:`docker` command is the ``cli_for`` the expectation about the API
  request that the command makes. The first names ``cli`` in its ``depends_on``, and the second does
  not. An expectation that is the CLI form of a defect violates what the defect violates.
- A requirement ``refines`` principles and can ``violate`` principles. A principle ``refines``
  principles.

A requirement that no expectation refines fails the build, except one that violates ``PRIN-STABLE``,
which fails it when no expectation is ``provisional_on`` it. So does a ceiling that bounds no
expectation, a rationale that no expectation links, and a link whose target is of the wrong type.
The following rules fail the build only for the specs that are not listed in ``LAX_DOCS`` in
``conf.py``: that a host requirement is needed by an expectation, that a detector is an exact form,
pins something or links a rationale, that a defect narrows or concerns an expectation, and that only
a defect narrows or concerns one. For the specs listed there, the build reports what these rules
find in ``_build/needs/schema_violations.json``. Every other rule in ``conf.py`` fails the build for
every spec.

.. toctree::
    :maxdepth: 2

    principles
    id-style-guide
