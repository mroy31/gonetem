# Spec Delta

## RENAMED Requirements

- FROM: `### Requirement: ospf command`
- TO: `### Requirement: ospfGraph command`

## MODIFIED Requirements

### Requirement: ospfGraph command
The console SHALL provide an `ospfGraph <node_name>` command that opens an interactive graph showing the OSPF area as seen by the named node, presented in the same in-terminal viewer style and terminal hand-off as the topology graph view. The former command name `ospf` SHALL NOT be accepted.

#### Scenario: View a running router
- **WHEN** a user runs `ospfGraph R1` on a project whose topology is running and R1 is a running router with OSPF configured
- **THEN** an interactive OSPF graph opens in the terminal
- **AND** quitting it returns the console to its prompt with the terminal restored as before

#### Scenario: Stopped or unknown node
- **WHEN** a user runs `ospfGraph` naming a node that does not exist, is stopped, or is not a router
- **THEN** the console reports an error and remains usable
- **AND** no graph opens

#### Scenario: Router without OSPF
- **WHEN** a user runs `ospfGraph` on a running router that has no OSPF instance configured
- **THEN** the console reports an error identifying that the node exposes no usable OSPF data

#### Scenario: Old command name rejected
- **WHEN** a user runs `ospf` at the console prompt
- **THEN** the console reports the command as unknown
