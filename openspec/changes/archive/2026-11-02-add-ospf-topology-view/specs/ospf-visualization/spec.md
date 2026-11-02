# Spec Delta

## Purpose

Visualize the link-state view of a routing engine: an interactive graph of the OSPF area as a named router currently sees it, obtained from that router's own link-state database.

## ADDED Requirements

### Requirement: ospf command
The console SHALL provide an `ospf <node_name>` command that opens an interactive graph showing the OSPF area as seen by the named node, presented in the same in-terminal viewer style and terminal hand-off as the topology graph view.

#### Scenario: View a running router
- **WHEN** a user runs `ospf R1` on a project whose topology is running and R1 is a running router with OSPF configured
- **THEN** an interactive OSPF graph opens in the terminal
- **AND** quitting it returns the console to its prompt with the terminal restored as before

#### Scenario: Stopped or unknown node
- **WHEN** a user runs `ospf` naming a node that does not exist, is stopped, or is not a router
- **THEN** the console reports an error and remains usable
- **AND** no graph opens

#### Scenario: Router without OSPF
- **WHEN** a user runs `ospf` on a running router that has no OSPF instance configured
- **THEN** the console reports an error identifying that the node exposes no usable OSPF data

### Requirement: Area view content
The graph SHALL be built from the named router's link-state database and SHALL label vertices with protocol identities only: routers by their router-id address and transit networks by their subnet prefix. Point-to-point adjacencies SHALL appear as direct edges between the two router vertices. The graph SHALL NOT display emulator node names.

#### Scenario: Transit network vertex
- **WHEN** the database contains routers attached to the same transit network
- **THEN** the graph shows one vertex labeled with that network's subnet prefix with an edge to every attached router

#### Scenario: Point-to-point edge
- **WHEN** the database contains a point-to-point adjacency between two routers
- **THEN** the graph shows a direct edge between the two router vertices without any network vertex

#### Scenario: Protocol identities only
- **WHEN** any vertex of the graph is displayed
- **THEN** its label is a router-id address or a subnet prefix, never an emulator node name

### Requirement: Queried router as root
The graph SHALL pin the vertex for the queried router's own router-id at the top of the hierarchical layout and SHALL highlight the edges attached to it.

#### Scenario: Root placement
- **WHEN** the user opens `ospf R1`
- **THEN** the vertex for R1's router-id is placed at the top of the hierarchy and its edges are visually highlighted

### Requirement: Link cost labels
Every edge SHALL display the OSPF cost of the corresponding link-state entry on its router-vertex side.

#### Scenario: Cost displayed
- **WHEN** an edge for a link-state entry with cost 10 is rendered
- **THEN** the label `10` is shown along the edge near the router vertex

### Requirement: Multi-area merging
When the database spans several areas, the view SHALL merge all areas into a single graph, representing a router that appears in multiple areas as one shared vertex.

#### Scenario: Areas joined at the border router
- **WHEN** a router originates router-link-state entries in area 0 and area 1
- **THEN** the graph contains a single vertex for that router-id connecting both areas' subgraphs

### Requirement: Snapshot semantics
The graph SHALL be a snapshot of the database read when the command runs; it SHALL NOT update while open.

#### Scenario: Re-read after a change
- **WHEN** the user quits the view, changes the network so the area changes, and runs `ospf` again
- **THEN** the newly opened graph reflects the changed area

### Requirement: Viewer interaction
The OSPF graph viewer SHALL support the same interactions as the topology graph viewer — scrolling when the graph exceeds the terminal, cycling node selection with the selected node's detail shown — and SHALL NOT offer re-pinning of the layout root.

#### Scenario: Scroll and select
- **WHEN** the user scrolls or cycles node selection while the OSPF graph is open
- **THEN** the view pans and moves the selection highlight as it does in the topology graph view

#### Scenario: No root picker
- **WHEN** the user presses the key that opens the root picker in the topology graph view
- **THEN** nothing happens in the OSPF graph view
