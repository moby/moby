##############
 Broken items
##############

Each item here breaks one rule, which tests/test_build.py expects the build to report. A document
that LAX_DOCS does not list is held to every rule, as this one is.

.. expectation:: A defect whose category is written
    :id: NET-TEST-WRITTEN-CATEGORY
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :violates: PRIN-TEST-ROOT
    :narrows: NET-TEST-CONTRACT
    :category: contract

.. req:: A requirement with the ID of an expectation
    :id: NET-TEST-NOT-A-REQ
    :source_kind: docs
    :source: The fixture
    :refines: PRIN-TEST-ROOT

.. expectation:: An expectation with the ID of a requirement
    :id: REQ-NET-TEST-NOT-AN-EXPECTATION
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. expectation:: An expectation that depends on none and on the OS
    :id: NET-TEST-NONE-AND-OS
    :observed_from: api
    :depends_on: none, os-linux
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. expectation:: A detector that does not say what it detects
    :id: NET-TEST-SAYS-NOTHING
    :observed_from: api
    :depends_on: none
    :platforms: linux

.. expectation:: Two vantage points without a separator
    :id: NET-TEST-OBSERVED-TWO
    :observed_from: task ctr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. expectation:: A vantage point outside the vocabulary
    :id: NET-TEST-OBSERVED-UNKNOWN
    :observed_from: api:manager
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. expectation:: An empty vantage point
    :id: NET-TEST-OBSERVED-EMPTY
    :observed_from: api,
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. expectation:: Two arrows
    :id: NET-TEST-OBSERVED-ARROWS
    :observed_from: task → ctr → node
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. expectation:: A written vantage
    :id: NET-TEST-VANTAGE-WRITTEN
    :observed_from: api
    :vantage: node
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. expectation:: Roles whose names the repository does not have
    :id: NET-TEST-ROLES
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

    :dockerd-flag:`--default-address-pools` is a daemon.json key, not a flag.
    :drvopt:`com.docker.network.bogus.driver` ends like an option. :ipt-chain:`bridge` and
    :nft-chain:`bridge` are strings of libnetwork, not chains. :repo-path:`/etc/passwd` and
    :repo-path:`../..` are outside the repository, and :repo-path:`API/openapi.yaml` has the wrong
    case. :api-op:`POST /networks/create trailing` and :api-op:`GET /networks/abc/bogus` are not
    operations. :dockerd-flag:`-X` is not a shorthand. :kconfig:`NOT_A_KCONFIG` is in no host
    requirement. :errno:`ENOTANERRNO` and :signal:`SIGNOPE` do not exist. :nft-table:`filter-FORWARD`
    is a chain. :drvopt:`com.docker.netwrok.brige.*` matches no option.

.. expectation:: A ceiling that is named but not linked
    :id: NET-TEST-CEILING-UNLINKED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

    It holds within :need:`[[title|lower]] ceiling ([[seconds]] s)
    <NET-CEIL-TEST>`.

.. expectation:: A host requirement of another platform
    :id: NET-TEST-WRONG-PLATFORM
    :observed_from: api
    :depends_on: none
    :platforms: windows
    :rationale: RAT-NET-TEST-WHY
    :needs: NET-HOST-TEST-KERNEL

.. expectation:: An expectation with a retired ID
    :id: NET-TEST-RETIRED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. expectation:: A vantage that a needextend writes
    :id: NET-TEST-EXTENDED-VANTAGE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. needextend:: NET-TEST-EXTENDED-VANTAGE
    :vantage: node

.. expectation:: An observed_from that a needextend changes
    :id: NET-TEST-EXTENDED-OBSERVED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. needextend:: NET-TEST-EXTENDED-OBSERVED
    :observed_from: no such vantage point

.. expectation:: A contract that refines the requirement with the ID of an expectation
    :id: NET-TEST-REFINES-NOT-A-REQ
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: NET-TEST-NOT-A-REQ

.. rationale:: A rationale with the ID of an expectation
    :id: NET-TEST-NOT-A-RATIONALE

    Its ID has no RAT- prefix.

.. principle:: A principle with the ID of an expectation
    :id: NET-TEST-NOT-A-PRINCIPLE

    Its ID has no PRIN- prefix.

.. expectation:: A CLI form that does not depend on the CLI
    :id: NET-TEST-CLI-WITHOUT-CLI
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :cli_for: NET-TEST-DETECTOR
    :rationale: RAT-NET-TEST-WHY, NET-TEST-NOT-A-RATIONALE

.. expectation:: A dependency on Linux on a Windows node
    :id: NET-TEST-OS-LINUX-ON-WINDOWS
    :observed_from: api
    :depends_on: os-linux
    :platforms: windows
    :rationale: RAT-NET-TEST-WHY

.. expectation:: A dependency on Windows on a Linux node
    :id: NET-TEST-OS-WINDOWS-ON-LINUX
    :observed_from: api
    :depends_on: os-windows
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. req:: A requirement without a source
    :id: REQ-NET-TEST-NO-SOURCE
    :source_kind: docs

.. expectation:: A contract that refines the requirement without a source
    :id: NET-TEST-REFINES-NO-SOURCE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-TEST-NO-SOURCE

.. expectation:: A contract that refines a promise that may change
    :id: NET-TEST-REFINES-UNSTABLE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-TEST-MAY-CHANGE

.. expectation:: Provisional on a promise that does not say it may change
    :id: NET-TEST-PROVISIONAL-ON-STABLE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :provisional_on: REQ-NET-TEST-PROMISE

.. hostreq:: A host requirement that no expectation needs
    :id: NET-HOST-TEST-UNNEEDED
    :kind: os
    :platforms: linux
    :kernel_options: CONFIG_BRIDGE

.. expectation:: A detector that narrows
    :id: NET-TEST-NARROWS-NOT-DEFECT
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :narrows: NET-TEST-DETECTOR
    :rationale: RAT-NET-TEST-WHY

.. expectation:: A detector that concerns
    :id: NET-TEST-CONCERNS-NOT-DEFECT
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :concerns: NET-TEST-DETECTOR
    :rationale: RAT-NET-TEST-WHY

.. expectation:: A defect that affects no expectation
    :id: NET-TEST-DEFECT-AFFECTS-NOTHING
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :violates: PRIN-TEST-ROOT

.. req:: A requirement that no expectation refines
    :id: REQ-NET-TEST-UNREFINED
    :source_kind: docs
    :source: The fixture

.. req:: A promise that may change, which no expectation is provisional on
    :id: REQ-NET-TEST-UNSTABLE-UNUSED
    :source_kind: docs
    :source: The fixture
    :violates: PRIN-STABLE

.. ceiling:: A ceiling that bounds nothing
    :id: NET-CEIL-TEST-UNUSED
    :seconds: 30

.. rationale:: A rationale that no expectation links
    :id: RAT-NET-TEST-UNUSED

    Nothing links it.

.. req:: A requirement with a written category and vantage
    :id: REQ-NET-TEST-CATEGORY
    :source_kind: docs
    :source: The fixture
    :category: contract
    :vantage: node

.. expectation:: A contract that refines the requirement with a written category
    :id: NET-TEST-REFINES-CATEGORY
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-TEST-CATEGORY

.. expectation:: An empty observed_from
    :id: NET-TEST-OBSERVED-NOTHING
    :observed_from:
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. expectation:: Words for an arrow
    :id: NET-TEST-OBSERVED-WORDS
    :observed_from: underlay to task
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. expectation:: A contract that a needextend gives a violation
    :id: NET-TEST-EXTENDED-CATEGORY
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-TEST-PROMISE

.. needextend:: NET-TEST-EXTENDED-CATEGORY
    :+violates: PRIN-TEST-ROOT

.. needextend:: REQ-NET-TEST-NO-SOURCE
    :vantage: node

.. expectation:: A provisional expectation whose category is written, as its links give it
    :id: NET-TEST-WRITTEN-MATCHING
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :provisional_on: REQ-NET-TEST-MAY-CHANGE
    :category: provisional

.. expectation:: The CLI form of a CLI expectation
    :id: NET-TEST-CLI-OF-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: NET-TEST-CLI-TARGET
    :rationale: RAT-NET-TEST-WHY

.. expectation:: A CLI expectation that another names as its API form
    :id: NET-TEST-CLI-TARGET
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY

.. needextend:: REQ-NET-TEST-UNREFINED
    :category: contract

.. expectation:: An observed_from that wraps
    :id: NET-TEST-OBSERVED-WRAPPED
    :observed_from: underlay → task or
        ctr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY
