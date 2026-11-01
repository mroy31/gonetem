# node-lifecycle

## Purpose

Define what an emulated node is (a Docker-based router/host/server/P4 switch or an
OVS switch), how nodes are created from topology options, and their runtime controls:
start, stop, restart and interface up/down, with state visible through status. Node
operations act on the server-side container; users interact with them through
project commands.

## Requirements

### Requirement: Node types and creation
Nodes SHALL be created for two families of types: `docker.<type>` node types (at
minimum router, host, server and p4sw, extensible by server configuration of custom
docker types) and `ovs` switches. Each node SHALL receive a unique short identifier
derived from its name, used to name its container and network attachments. Unknown
node types SHALL fail creation.

#### Scenario: Custom docker type accepted
- **WHEN** a topology uses `docker.<type>` for a node type defined in the server
  configuration
- **THEN** the node is created as a Docker container running that type's image

#### Scenario: Node options take effect at creation
- **WHEN** a node is created with topology options (IPv6, MPLS, VRFs, VRRP groups,
  volume binds, custom image)
- **THEN** its runtime environment is prepared so those features are usable once the
  node starts

#### Scenario: Invalid node type
- **WHEN** a topology declares a node type outside `docker.<type>` and `ovs`
- **THEN** validation rejects the topology before any node is created

### Requirement: Start a node
Starting a node SHALL start its underlying container and load its saved
configuration (running the type's configured load commands), returning any messages
the load step emitted. Starting an unknown node SHALL fail.

#### Scenario: Start a stopped node
- **WHEN** a client starts a node that exists in a running topology and is stopped
- **THEN** the node becomes running with its configuration applied

#### Scenario: Start ignored before run
- **WHEN** a client starts a node while the topology itself is not running
- **THEN** the call succeeds without starting the node and the request is ignored

### Requirement: Stop and restart a node
Stopping a node SHALL stop its container while keeping the node and its links part of
the topology. Restart SHALL stop then start a node.

#### Scenario: Restart reapplies configuration
- **WHEN** a client restarts a running node
- **THEN** the node stops and comes back running with its configuration reloaded

#### Scenario: Unknown node
- **WHEN** a client stops or restarts a name not present in the topology
- **THEN** the request fails with a node-not-found error

### Requirement: Interface up and down
The server SHALL set an emulated node interface (addressed by node name and interface
index, where index N maps to the node's `ethN` interface) to up or down while the
node runs, and SHALL fail when the targeted interface does not exist on that node.

#### Scenario: Admin down a link endpoint
- **WHEN** a client sets `router1.2` down
- **THEN** the corresponding interface reports DOWN in project status
- **AND** traffic through it stops until it is set up again

#### Scenario: Missing interface
- **WHEN** a client sets state on an interface index the node has not attached
- **THEN** the request fails with an error

### Requirement: Node state reporting
Project status SHALL report, for every node of the topology, whether it is running
and, while the topology runs, the up/down state of its attached interfaces.

#### Scenario: Stopped node in status
- **WHEN** a node is stopped in a running topology
- **THEN** project status lists it with running false

### Requirement: Switch nodes
OVS switch nodes SHALL be managed through a per-project OVS instance started with the
topology; switch forwarding state lives in that instance, and destroying the project
or reloading the topology SHALL release the switch's configuration.

#### Scenario: Switch follows topology lifecycle
- **WHEN** a topology containing ovs nodes is run then closed
- **THEN** the switch instance and its bridges are removed with the project state
