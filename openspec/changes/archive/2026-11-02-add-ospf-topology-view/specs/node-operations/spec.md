# Spec Delta

## ADDED Requirements

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
