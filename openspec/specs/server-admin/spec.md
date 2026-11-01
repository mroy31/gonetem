# server-admin

## Purpose

Define the administrative surface of the `gonetem-server`: its configuration and
listener (including optional mutual TLS), version reporting, acquisition of the
emulated node images it depends on, cleanup of containers abandoned by crashed
sessions, and the server-side definition of node types.

## Requirements

### Requirement: Server configuration
The server SHALL load its configuration from a YAML file (listen address, TLS
options, work directory, docker operation timeout, node type definitions and switch
image), with safe built-in defaults so it starts with a minimal file. The work
directory SHALL host every project's temporary working copy.

#### Scenario: Minimal configuration
- **WHEN** the server starts with only listen address, work directory and node image
  entries configured
- **THEN** it starts successfully, listening on the configured address

### Requirement: Optional mutual TLS
The server SHALL serve plain gRPC by default and, when TLS is enabled in its
configuration, serve over TLS requiring and verifying client certificates against the
configured CA.

#### Scenario: TLS enabled rejects unauthenticated clients
- **WHEN** TLS is enabled and a client connects without a client certificate
- **THEN** the handshake fails and the client cannot reach the API

### Requirement: Version reporting
The server SHALL report its version to clients.

#### Scenario: Version query
- **WHEN** a client requests the server version
- **THEN** the response contains the running server's version string

### Requirement: Node image acquisition
The server SHALL pull, on demand, the container images it needs for the node types it
provides (routers, hosts, servers, P4 switches, and the switch image), using the
configured image names and defaulting an untagged image to the server's version. Each
image SHALL be reported through the stream as start, then success or error, and a
failing image SHALL not abort the remaining pulls.

#### Scenario: Pull all images
- **WHEN** a client invokes the image pull
- **THEN** one start event per configured image streams, each followed by success or
  its error, and the stream ends after the last image

### Requirement: Orphaned container cleanup
The server SHALL remove every gonetem-managed container whose owning project is not
currently open, stopping then deleting each, and SHALL surface per-container failures.
The console SHALL ask for confirmation before invoking cleanup.

#### Scenario: Clean after a crash
- **WHEN** containers exist from a project that is no longer open and a client runs
  cleanup
- **THEN** those containers are stopped and removed
- **AND** containers of currently open projects are left untouched

### Requirement: Node type definitions
Node types SHALL be defined by server configuration rather than code: for each type,
its image, optional extra volume binds, console/shell commands, load/save config
commands, and the list of configuration files and folders (with labels) that node
configuration read and save exchange. Additional docker node types SHALL be
addable through an extension list.

#### Scenario: New docker node type
- **WHEN** the server configuration defines an extra docker node type with its
  commands and configuration files
- **THEN** topologies can declare `docker.<type>` nodes that start, load and save
  their configuration like built-in types
