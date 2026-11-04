# Spec Delta

## MODIFIED Requirements

### Requirement: Area view content
The graph SHALL be built from the named router's link-state database and SHALL label vertices with protocol identities only: routers by their router-id address, and transit and stub networks by their subnet prefix. Point-to-point adjacencies SHALL appear as direct edges between the two router vertices, whether or not the same pair of routers is also linked through a stub-network vertex. The graph SHALL NOT display emulator node names.

#### Scenario: Transit network vertex
- **WHEN** the database contains routers attached to the same transit network
- **THEN** the graph shows one vertex labeled with that network's subnet prefix with an edge to every attached router

#### Scenario: Point-to-point edge
- **WHEN** the database contains a point-to-point adjacency between two routers
- **THEN** the graph shows a direct edge between the two router vertices, the adjacency itself creating no network vertex
- **AND** the edge is still shown when those routers also share a stub-network vertex for the same subnet

#### Scenario: Protocol identities only
- **WHEN** any vertex of the graph is displayed
- **THEN** its label is a router-id address or a subnet prefix, never an emulator node name

## ADDED Requirements

### Requirement: Stub network leaves
The graph SHALL represent each OSPF stub-network link in the database as a leaf vertex labeled with its subnet prefix, adjacent to its advertising router, except links whose prefix is /32, which SHALL NOT appear. The view SHALL offer no option or key to change this.

#### Scenario: Stub subnet displayed
- **WHEN** the database contains a router originating a stub-network link to 10.0.9.0/24
- **THEN** the graph shows a vertex labeled 10.0.9.0/24 connected to that router's vertex

#### Scenario: Loopback stubs hidden
- **WHEN** the database contains a router originating a stub-network link with prefix /32
- **THEN** the graph contains no vertex for that prefix
- **AND** no viewer interaction can make it appear

#### Scenario: Shared stub merges into one vertex
- **WHEN** two routers each advertise a stub-network link to the same 10.0.24.0/24 subnet
- **THEN** the graph shows a single 10.0.24.0/24 vertex with one edge to each advertising router

#### Scenario: Transit identity wins over stub
- **WHEN** a subnet appears in the database both as a transit network and as a stub-network link
- **THEN** the graph shows one vertex for that subnet keeping its transit identity
