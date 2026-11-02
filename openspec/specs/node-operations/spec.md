# node-operations

## Purpose

Define the interactive and file-level operations a client can perform against a
running node: opening a console or shell, streaming an exec session, capturing
traffic on an interface, copying files in and out, and reading or persisting node
configuration files.

## Requirements

### Requirement: Console and shell access
For a running node, the server SHALL return the command that opens a terminal into
that node: the node type's console command (e.g. a routing shell on routers, a
switch CLI on P4 switches, a shell on hosts/servers) or, when shell mode is
requested, the type's shell command. The client launches the returned command in a
local terminal.

#### Scenario: Console of the right kind
- **WHEN** a client asks for the console command of a running router node without
  shell mode
- **THEN** the returned command enters the router's configuration shell rather than
  a plain shell

#### Scenario: Unknown node
- **WHEN** a console command is requested for a node that does not exist
- **THEN** the request fails with a node-not-found error

### Requirement: Streaming exec
The server SHALL support a bidirectional exec session on a node: the client opens
with a command plus tty flag and terminal size, sends stdin data frames, may send
terminal resize frames, and receives stdout and error frames until a close frame.
The first client message SHALL be a command frame.

#### Scenario: Run a command non-interactively
- **WHEN** a client execs a short-lived command without tty
- **THEN** the command's output streams back and the session ends with a close frame

#### Scenario: Wrong first frame
- **WHEN** the first message of an exec session is not a command frame
- **THEN** the server replies with an error frame and the session ends

#### Scenario: Exec on unknown node
- **WHEN** the command frame names a project or node that does not exist
- **THEN** the server sends an error frame naming the missing project or node

### Requirement: One-shot exec
The server SHALL support running a command on a running node without a tty and without interactive input, returning the command's captured standard output and its exit status to the client in a single request/response exchange. A non-zero exit status SHALL be delivered as a normal response carrying the exit status, not as a transport failure.

#### Scenario: Capture command output
- **WHEN** a client requests a one-shot exec of a short-lived command on a running node
- **THEN** the response contains the command's full standard output and exit status 0

#### Scenario: Command fails inside the node
- **WHEN** the executed command exits with a non-zero status
- **THEN** the response reports the non-zero exit status to the client

#### Scenario: Unknown or stopped target
- **WHEN** a one-shot exec names a project or node that does not exist, or a node that is not running
- **THEN** the request fails with an error naming the missing or unavailable target

### Requirement: Interface capture
Capturing on a node interface SHALL stream the captured traffic of that interface
(node interface addressed by index) back to the client, which may display or pipe it
to an analyzer. Unknown projects and nodes SHALL be reported as error frames on the
capture stream. Capture of a non-up interface SHALL be refused with an error.

#### Scenario: Capture a link
- **WHEN** a client captures on `<node>.<ifIndex>` while traffic flows
- **THEN** an OK frame is followed by streamed capture data until the client ends
  the session

#### Scenario: Capture a down interface
- **WHEN** a client requests capture on an interface that is down
- **THEN** the server reports an error instead of starting a capture

### Requirement: File copy to and from a node
The server SHALL copy a file out of a node (streaming its bytes back to the client)
and into a node (receiving an init frame naming the node and destination path, then
data frames, ending with an acknowledgement). Copies between two nodes in a single
operation SHALL NOT be supported.

#### Scenario: Copy out
- **WHEN** a client requests a file from a node path
- **THEN** the file bytes stream back and the server-side temporary copy is removed

#### Scenario: Copy in
- **WHEN** a client streams a local file toward a node destination path
- **THEN** the server stages it on a temporary file, places it in the node, and
  acknowledges OK

#### Scenario: Node-to-node copy
- **WHEN** a client attempts a copy with both source and destination being nodes
- **THEN** the console rejects the request as unsupported

### Requirement: Read node configuration files
The server SHALL return the named configuration files of a node, tagged with their
source: `RUNNING` when read from the live container, `ARCHIVE` when read from saved
project files (node stopped).

#### Scenario: Read from a running node
- **WHEN** config files are requested from a running node
- **THEN** the response lists that node type's configuration files read from the
  container with source RUNNING

#### Scenario: Read from the project archive
- **WHEN** config files are requested from a stopped node
- **THEN** the response falls back to the files saved in the project with source
  ARCHIVE

### Requirement: Persist node configuration
Saving a node SHALL run its type's configured save commands and then copy its
declared configuration files and folders into the project `configs/` directory,
keyed by node and file label. Only running nodes save; the project-level save
composes per-node saves.

#### Scenario: Save router configuration
- **WHEN** a save runs while a router node is running
- **THEN** its configuration files appear under the project configs directory for
  later reload and archive retrieval
