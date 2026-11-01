# topology-lifecycle

## Purpose

Define the declarative topology model (`network.yml`), its validation rules, and the
orchestration that turns a validated topology into running network state: running,
reloading, starting/stopping all nodes, plus bridges and the optional management
network. The topology file is the source of truth edited by users and written back by
live operations when sync is requested.

## Requirements

### Requirement: Topology file schema
The topology SHALL be defined in `network.yml` with four sections: `nodes` (map of
name to node options), `links` (list of point-to-point links), `bridges` (map of
linux-bridge definitions) and `mgntnet` (optional management network). Node options
SHALL include: `type`, `image`, `launch` (default true), `ipv6` (default true),
`mpls`, `vrfs`, `vrrps`, `volumes` and per-node `mgnt` attachment.

#### Scenario: Defaults applied
- **WHEN** a node is declared without `launch` or `ipv6`
- **THEN** the node defaults to launching at topology run and having IPv6 enabled

### Requirement: Topology validation
The server SHALL validate a topology before any state is created, report every
violation it finds in one result, and reject invalid topologies. Validation SHALL
cover node definition sanity, node option/type compatibility, volume bind syntax and
host existence, link peer sanity, bridge host interface existence, and QoS parameter
consistency.

#### Scenario: Valid topology passes check
- **WHEN** a client requests a check of a valid `network.yml`
- **THEN** the response reports OK

#### Scenario: Invalid topology reports all errors
- **WHEN** a client checks a topology containing several invalid entries
- **THEN** the response returns ERROR with a message enumerating every violation

#### Scenario: Node sanity rules
- **WHEN** a node name fails the `^\w+$` pattern or is duplicated, or a node type is
  outside `docker.<type>` and `ovs`, or an `ovs` switch name exceeds 10 characters
- **THEN** each such defect is reported as a violation

#### Scenario: Node option and type compatibility
- **WHEN** mpls or vrfs are set on an `ovs` node, or vrrps are set on a node that is
  not `docker.router` or carry an invalid CIDR address
- **THEN** each such combination is reported as a violation

#### Scenario: Volume bind rules
- **WHEN** a volume bind is not `hostPath:containerPath` or its host path does not
  exist
- **THEN** the bind is reported as a violation

#### Scenario: Link peer sanity
- **WHEN** a link peer is not formatted `<node>.<ifIndex>`, names a node that does
  not exist, reuses a peer already used by another link or bridge interface, or a
  link lists the same peer twice
- **THEN** each such defect is reported as a violation

#### Scenario: QoS consistency rules
- **WHEN** delay, jitter or loss is negative, loss exceeds 100, jitter or rate is set
  with a zero delay, or buffer is set with a zero rate
- **THEN** each such defect is reported as a violation

### Requirement: Run the topology
Running a topology SHALL start the project's switch instance, create the management
network when enabled, start in parallel the nodes marked to launch, create all links,
set up bridges, then configure interfaces and load each running node's saved
configuration. Node work SHALL be bounded concurrency. Run SHALL report progress
(counts first, then per-node/link/bridge steps) and SHALL finish by delivering any
per-node warning messages produced during configuration load.

#### Scenario: Run a loaded topology
- **WHEN** a client runs a project whose topology is loaded and not running
- **THEN** all launchable nodes end up running with their links established and
  configurations loaded
- **AND** progress messages stream node/link/bridge counts and completion steps

#### Scenario: Run is idempotent while running
- **WHEN** a client runs a topology that is already running
- **THEN** the server performs no changes and the call succeeds

#### Scenario: Nodes excluded from startup
- **WHEN** the topology contains nodes declared with `launch: false`
- **THEN** those nodes are not started by run and remain stopped until started
  individually

### Requirement: Reload the topology
Reloading SHALL destroy the current network state and re-read `network.yml` from the
server-side project directory; if the topology was running before the reload, it
SHALL be run again with the new definition.

#### Scenario: Edit then reload
- **WHEN** a client writes a modified `network.yml` and reloads a running project
- **THEN** the previous state is fully torn down and the new topology is run
- **AND** an invalid new file fails the reload with validation errors

### Requirement: Start and stop all nodes
The server SHALL support starting all not-running nodes (each with its configuration
load) and stopping all running nodes, without destroying the links or the topology
definition.

#### Scenario: Stop then start all
- **WHEN** a client stops all nodes of a running topology then starts them again
- **THEN** all nodes end running and links remain established

### Requirement: Bridge attachment
Each declared bridge SHALL create a linux bridge on the server that joins the named
host physical interface with the declared node interfaces, each attached through a
veth endpoint named after the node and interface index.

#### Scenario: Bridge spanning host and emulated nodes
- **WHEN** a topology declares a bridge with a server host interface and node
  interface peers
- **THEN** the bridge exists on the server with the host interface and one veth per
  node peer attached and up

### Requirement: Management network
When `mgntnet` is enabled, the server SHALL create a management bridge carrying the
configured address, and every node declaring a `mgnt` attachment SHALL receive a
dedicated `mgnt` interface wired to that bridge with its configured address.

#### Scenario: Node reaches management network
- **WHEN** a running topology has `mgntnet` enabled and a node opts into `mgnt`
- **THEN** that node has a `mgnt` interface attached to the management bridge with
  the configured address

### Requirement: Network file access
The server SHALL expose read and write access to the project's current `network.yml`
in the server-side working directory, which the console uses to let the user edit the
topology with an external editor.

#### Scenario: Read-modify-write cycle
- **WHEN** a client reads the network file, writes modified content back, and reloads
- **THEN** subsequent reads and the reload observe the written content
