#############
 Valid items
#############

Every item here meets every rule, so that the build reports nothing about this document.

.. req:: The daemon keeps its promise
    :id: REQ-NET-TEST-PROMISE
    :source_kind: docs
    :source: The fixture
    :refines: PRIN-TEST-ROOT

.. req:: The daemon keeps a promise that may change
    :id: REQ-NET-TEST-MAY-CHANGE
    :source_kind: docs
    :source: The fixture, which says that the behavior may change
    :violates: PRIN-STABLE

.. rationale:: A change to a detector of the fixture should be deliberate
    :id: RAT-NET-TEST-WHY

    Tests rely on it.

.. hostreq:: The kernel of the fixture
    :id: NET-HOST-TEST-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: BRIDGE

.. ceiling:: The fixture converges in time
    :id: NET-CEIL-TEST
    :seconds: 30

.. needextend:: type == "expectation" and docname == "network/good" and "ctr" in vantage
    :+needs: NET-HOST-TEST-KERNEL

.. expectation:: Traffic from the underlay reaches a task or a container
    :id: NET-TEST-CONTRACT
    :observed_from: underlay → task or ctr (on the same node)
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-TEST-PROMISE
    :within: NET-CEIL-TEST

    The names that these roles hold are where the checks look for them:
    :dockerd-flag:`--mtu`, :drvopt:`com.docker.network.driver.mtu`, :ipt-chain:`DOCKER-USER`,
    :repo-path:`api/openapi.yaml`, :repo-path:`integration/network/bridge/`, :api-op:`POST
    /networks/create` and :api-op:`GET /networks/{id}`, :dockerd-flag:`--debug`, :dockerd-flag:`-D`,
    :kconfig:`CONFIG_BRIDGE`, :errno:`EPERM`, :errno:`ENOTCONN`, :signal:`SIGTERM`, :signal:`SIGSEGV`, :nft-table:`docker-bridges` and
    :nft-chain:`filter-FORWARD`, :drvopt:`com.docker.network.bridge.*`. It holds within
    :need:`NET-CEIL-TEST`.

.. expectation:: The API answers
    :id: NET-TEST-DETECTOR
    :observed_from: api, log
    :depends_on: none
    :platforms: linux
    :rationale: RAT-NET-TEST-WHY, RAT-NET-TEST-OUTER, RAT-NET-TEST-INNER

    The API answers a request.

.. expectation:: The daemon breaks its promise
    :id: NET-TEST-DEFECT
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :violates: PRIN-TEST-ROOT
    :narrows: NET-TEST-CONTRACT

    The daemon does what the fixture does not say.

.. expectation:: The daemon keeps a promise that may change
    :id: NET-TEST-PROVISIONAL
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :provisional_on: REQ-NET-TEST-MAY-CHANGE

    The daemon does what the fixture says, for now.
