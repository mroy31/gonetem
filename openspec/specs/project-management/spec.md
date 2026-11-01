# project-management

## Purpose

Define the lifecycle and file format of gonetem projects: a project is a `.gnet`
archive that the server opens into a live, in-memory working copy, and that can be
saved back, listed, inspected and closed. This capability covers the `.gnet` format
itself, project open/close/save/list/status behavior, and the console commands that
operate on project files locally.

## Requirements

### Requirement: Project file format
A project file SHALL use the `.gnet` extension and be a gzip-compressed tar archive
containing at least a `network.yml` topology file, and optionally a `configs/`
directory holding saved node configuration files. The console SHALL refuse to create
or open files without the `.gnet` extension.

#### Scenario: Create an empty project
- **WHEN** the user runs `gonetem-console create <path>.gnet`
- **THEN** the archive is created containing a `network.yml` with empty `nodes`,
  `links` and `bridges` sections
- **AND** creation fails if the target path already exists

#### Scenario: Extract a project to disk
- **WHEN** the user runs `gonetem-console extract <path>.gnet <dir>` with `<dir>` an
  existing directory
- **THEN** the archive contents are unpacked into `<dir>`
- **AND** extraction fails with an error if the destination directory does not exist

### Requirement: Open a project
The server SHALL open a project by unpacking the uploaded archive into a temporary
directory under its configured work directory, loading and validating the topology,
assigning a short unique project identifier, and registering the project by a
user-supplied display name. Opening does not start the topology.

#### Scenario: Successful open
- **WHEN** a client opens a valid `.gnet` archive with an unused name
- **THEN** the response contains OK status and the new project id
- **AND** the project appears in the open-project list with its name and open time

#### Scenario: Duplicate project name
- **WHEN** a client opens a project with a name that is already open on the server
- **THEN** the response returns an ERROR status advising the use of the `--name`
  option to open it anyway

#### Scenario: Corrupt archive or invalid topology
- **WHEN** a client opens data that is not a valid gzip/tar archive, or whose
  topology fails validation
- **THEN** the open fails with an error
- **AND** the temporary directory is removed and no project is registered

### Requirement: Project list and status
The server SHALL expose the set of currently open projects (id, name, opened-at) and
a per-project status including whether the topology is running, and for each node
whether it is running and the state of its interfaces.

#### Scenario: List projects
- **WHEN** a client requests the project list
- **THEN** every open project is returned with id, name and open timestamp

#### Scenario: Status includes node and interface state
- **WHEN** a client requests the status of a running project
- **THEN** the response reports the project running flag and, per node, its running
  state and the up/down state of each attached interface

#### Scenario: Unknown project id
- **WHEN** a client requests status for an id that is not open
- **THEN** the request fails with a project-not-found error

### Requirement: Save a project
Saving a project SHALL collect each running node's configuration into the project's
`configs/` directory and return the whole server-side project directory as a `.gnet`
archive stream. Nodes that are not running SHALL be skipped.

#### Scenario: Save running project
- **WHEN** a client saves an open project
- **THEN** the archive streams back containing the current `network.yml` and the
  configuration of every running node
- **AND** progress messages report the node count and per-node save steps

#### Scenario: Save unknown project
- **WHEN** a client saves an id that is not open
- **THEN** the save fails with a project-not-found error

### Requirement: Close a project
Closing a project SHALL destroy its topology (stop and remove nodes, bridges,
management network and switch instances), delete the server-side temporary directory,
and unregister the project. Closing shall complete even if individual resources fail
to release, reporting those failures.

#### Scenario: Close a running project
- **WHEN** a client closes an open project
- **THEN** its nodes and network state are removed from the server
- **AND** the temporary working directory is deleted
- **AND** progress messages report per-node and per-bridge teardown

#### Scenario: Close twice
- **WHEN** a client closes a project that is not open
- **THEN** the close fails with a project-not-found error
