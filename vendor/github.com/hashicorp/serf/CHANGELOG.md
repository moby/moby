## 0.7.1 (UNRELEASED)

IMRPOVEMENTS:

 * Serf's Go dependencies are now vendored using govendor. [GH-383]

BUG FIXES:

 * Updated memberlist to pull in a fix for leaking goroutines when performing TCP fallback pings. This affected users with frequent UDP connectivity problems. [GH-381]

## 0.7 (December 21, 2015)

FEATURES:

 * Added new network tomography subsystem that computes network coordinates for
   nodes in the cluster which can be used to estimate network round trip times
   between any two nodes; exposes new `GetCoordinate` API as as well as a
   a new `serf rtt` command to query RTT interactively

IMPROVEMENTS:

 * Added support for configuring query request size and query response size [GH-346]
 * Syslog messages are now filtered by the configured log-level
 * New `statsd_addr` for sending metrics via UDP to statsd
 * Added support for sending telemetry to statsite
 * `serf info` command now displays event handlers [GH-312]
 * Added a `MemberLeave` message to the `EventCh` for a force-leave so higher-
   level applications can handle the leave event
 * Lots of documentation updates

BUG FIXES:

 * Fixed updating cached protocol version of a node when an update event
   fires [GH-335]
 * Fixed a bug where an empty remote state message would cause a crash in
   `MergeRemoteState`

## 0.6.4 (Febuary 12, 2015)

IMPROVEMENTS:

 * Added merge delegate to Serf library to support application
   specific logic in cluster merging.
 * `SERF_RPC_AUTH` environment variable can be used in place of CLI flags.
 * Display if encryption is enabled in Serf stats
 * Improved `join` behavior when using DNS resolution

BUG FIXES:

 * Fixed snapshot file compaction on Windows
 * Fixed device binding on Windows
 * Fixed bug with empty keyring
 * Fixed parsing of ports in some cases
 * Fixing stability issues under high churn

MISC:

* Increased user event size limit to 512 bytes (previously 256)

## 0.6.3 (July 10, 2014)

IMPROVEMENTS:

* Added `statsite_addr` configuration to stream to statsite

BUG FIXES:

* Fixed issue with mDNS flooding when using IPv4 and IPv6
* Fixed issue with reloading event handlers

MISC:

* Improved failure detection reliability under load
* Reduced fsync() use in snapshot file
* Improved snapshot file performance
* Additional logging to help debug flapping

## 0.6.2 (June 16, 2014)

IMPROVEMENTS:

* Added `syslog_facility` configuration to set facility

BUG FIXES:

* Fixed memory leak in in-memory stats system
* Fixed issue that would cause syslog to deadlock

MISC:

* Fixed missing prefixes on some log messages
* Docs fixes

## 0.6.1 (May 29, 2014)

BUG FIXES:

* On Windows, a "failed to decode request header" error will no
  longer be shown on every RPC request.

* Avoiding holding a lock which can cause monitor/stream commands to
  fail when an event handler is blocking

* Fixing conflict response decoding errors

IMPROVEMENTS:

* Improved agent CLI usage documentation

* Warn if an event handler is slow, potentially blocking other events

## 0.6.0 (May 8, 2014)

FEATURES:

 * Support for key rotation when using encryption. This adds a new
 `serf keys` command, and a `-keyring-file` configuration. Thanks
 to @ryanuber.

 * New `-tags-file` can be specified to persist changes to tags made
 via the RPC interface. Thanks to @ryanuber.

 * New `serf info` command to provide operator debugging information,
 and to get info about the local node.

 * Adding `-retry-join` flag to agent which enables retrying the join
 until success or `-retry-max` attempts have been made.

IMPROVEMENTS:

 * New `-rejoin` flag can be used along with a snapshot file to
 automatically rejoin a cluster.

 * Agent uses circular buffer to invoke handlers, guards against unbounded
 output lengths.

 * Adding support for logging to syslog

 * The SERF_RPC_ADDR environment variable can be used instead of the
 `-rpc-addr` flags. Thanks to @lalyos [GH-209].

 * `serf query` can now output the results in a JSON format.

 * Unknown configuration directives generate an error [GH-186].
 Thanks to @vincentbernat.

BUG FIXES:

 * Fixing environmental variables with invalid characters. [GH-200].
 Thanks to @arschles.

 * Fixing issue with tag changes with hard restart before
   failure detection.

 * Fixing issue with reconnect when using dynamic ports.

MISC:

 * Improved logging of various error messages

 * Improved debian packaging. Thanks to @vincentbernat.

## 0.5.0 (March 12, 2014)

FEATURES:

 * New `query` command provides a request/response mechanism to do realtime
 queries across the cluster. [GH-139]

 * Automatic conflict resolution. Serf will detect name conflicts, and use an
 internal query to determine which node is in the minority and perform a shutdown.
 [GH-167] [GH-119]

 * New `reachability` command can be used to help diagnose network and configuration
 issues.

 * Added `member-reap` event to get notified of when Serf removes a failed or left
 node from the cluster. The reap interval is controlled by `reconnect_timeout` and
 `tombstone_timeout` respectively. [GH-172]

IMPROVEMENTS:

 * New Recipes section on the site to share Serf tips. Thanks to @ryanuber. [GH-177]

 * `members` command has new `-name` filter flag. Thanks to @ryanuber [GH-164]

 * New RPC command "members-filtered" to move filtering logic to the agent.
 Thanks to @ryanuber. [GH-149]

 * `reconnect_interval` and `reconnect_timeout` can be provided to configure
 agent behavior for attempting to reconnect to failed nodes. [GH-155]

 * `tombstone_interval` can be provided to configure the reap time for nodes
 that have gracefully left. [GH_172]

 * Agent can be provided `rpc_auth` config to require that RPC is authenticated.
 All commands can take a `-rpc-auth` flag now. [GH-148]

BUG FIXES:

 * Fixed config folder in Upstart script. Thanks to @llchen223. [GH-174]

 * Event handlers are correctly invoked when BusyBox is the shell. [GH-156]

 * Event handlers were not being invoked with the correct SERF_TAG_* values
 if tags were changed using the `tags` command. [GH-169]

MISC:

  * Support for protocol version 1 (Serf 0.2) has been removed. Serf 0.5 cannot
  join a cluster that has members running version 0.2.

## 0.4.5 (February 25, 2014)

FEATURES:

 * New `tags` command is available to dynamically update tags without
 reloading the agent. Thanks to @ryanuber. [GH-126]

IMPROVEMENTS:

 * Upstart receipe logs output thanks to @breerly [GH-128]

 * `members` can filter on any tag thanks to @hmrm [GH-124]

 * Added vagrant demo to make a simple cluster

 * `members` now columnizes the output thanks to @ryanuber [GH-138]

 * Agent passes its own environment variables through thanks to @mcroydon [GH-142]

 * `-iface` flag can be used to bind to interfaces [GH-145]

BUG FIXES:

 * -config-dir would cause protocol to be set to 0 if there are no
 configuration files in the directory [GH-129]

 * Event handlers can filter on 'member-update'

 * User event handler appends new line, this was being omitted

## 0.4.1 (February 3, 2014)

IMPROVEMENTS:

 * mDNS service uses the advertise address instead of bind address

## 0.4.0 (January 31, 2014)

FEATURES:

 * Static `role` has been replaced with dynamic tags. Each agent can have
 multiple key/value tags associated using `-tag`. Tags can be updated using
 a SIGHUP and are advertised to the cluster, causing the `member-update` event
 to be triggered. [GH-111] [GH-98]

 * Serf can automatically discover peers uing mDNS when provided the `-discover`
 flag. In network environments supporting multicast, no explicit join is needed
 to find peers. [GH-53]

 * Serf collects telemetry information and simple runtime profiling. Stats can
 be dumped to stderr by sending a `USR1` signal to Serf. Windows users must use
 the `BREAK` signal instead. [GH-103]

 * `advertise` flag can be used to set an advertise address different
 from the bind address. Used for NAT traversal. Thanks to @benagricola [GH-93]

 * `members` command now takes `-format` flag to specify either text or JSON
 output. Fixed by @ryanuber [GH-97]

IMPROVEMENTS:

 * User payload always appends a newline when invoking a shell script

 * Severity of "Potential blocking operation" reduced to debug to prevent
 spurious messages on slow or busy machines.

BUG FIXES:

 * If an agent is restarted with the same bind address but new name, it
 will not respond to the old name, causing the old name to enter the
 `failed` state, instead of having duplicate entries in the `alive` state.

 * `leave_on_interrupt` set to false when not specified, if
 any config file is provided. This flag is deprecated for
 `skip_leave_on_interrupt` instead. [GH-94]

MISC:

  * `-role` configuration has been deprecated in favor of `-tag role=foo`.
  The flag is still supported but will generate warnings.

  * Support for protocol version 0 (Serf 0.1) has been removed. Serf 0.4 cannot
  join a cluster that has members running version 0.1.

## 0.3.0 (December 5, 2013)

FEATURES:

  * Dynamic port support, cluster wide consistent config not necessary
  * Snapshots to automaticaly rejoin cluster after failure and prevent replays [GH-84] [GH-71]
  * Adding `profile` config to agent, to support WAN, LAN, and Local modes
  * MsgPack over TCP RPC protocol which can be used to control Serf, send events, and
  receive events with low latency.
  * New `leave` CLI command and RPC endpoint to control graceful leaves
  * Signal handling is controlable, graceful leave behavior on SIGINT/SIGTERM
  can be specified
  * SIGHUP can be used to reload configuration

IMPROVEMENTS:

  * Event handler provides lamport time of user events via SERF_USER_LTIME [GH-68]
  * Memberlist encryption overhead has been reduced
  * Filter output of `members` using regular expressions on role and status
  * `replay_on_join` parameter to control replay with `start_join`
  * `monitor` works even if the client is behind a NAT
  * Serf generates warning if binding to public IP without encryption

BUG FIXES:

  * Prevent unbounded transmit queues [GH-78]
  * IPv6 addresses can be bound to [GH-72]
  * Serf join won't hang on a slow/dead node [GH-70]
  * Serf Leave won't block Shutdown [GH-1]

## 0.2.1 (November 6, 2013)

BUG FIXES:

  * Member role and address not updated on re-join [GH-58]

## 0.2.0 (November 1, 2013)

FEATURES:

  * Protocol versioning features so that upgrades can be done safely.
    See the website on upgrading Serf for more info.
  * Can now configure Serf with files or directories of files by specifying
    the `-config-file` and/or `-config-dir` flags to the agent.
  * New command `serf force-leave` can be used to force a "failed" node
    to the "left" state.
  * Serf now supports message encryption and verification so that it can
    be used on untrusted networks [GH-25]
  * The `-join` flag on `serf agent` can be used to join a cluster when
    starting an agent. [GH-42]

IMPROVEMENTS:

  * Random staggering of periodic routines to avoid cluster-wide
    synchronization
  * Push/Pull timer automatically slows down as cluster grows to avoid
    congestion
  * Messages are compressed to reduce bandwidth utilization
  * `serf members` now provides node roles in output
  * Joining a cluster will no longer replay all the old events by default,
    but it can using the `-replay` flag.
  * User events are coalesced by default, meaning duplicate events (by name)
    within a short period of time are merged. [GH-8]

BUG FIXES:

  * Event handlers work on Windows now by executing commands through
    `cmd /C` [GH-37]
  * Nodes that previously left and rejoin won't get stuck in 'leaving' state.
    [GH-18]
  * Fixing alignment issues on i386 for atomic operations [GH-20]
  * "trace" log level works [GH-31]

## 0.1.1 (October 23, 2013)

BUG FIXES:

  * Default node name is outputted when "serf agent" is called with no args.
  * Remove node from reap list after join so a fast re-join doesn't lose the
    member.

## 0.1.0 (October 23, 2013)

* Initial release
