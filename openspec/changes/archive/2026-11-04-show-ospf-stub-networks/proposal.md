# Proposal

## Why

The `ospfGraph` view currently drops OSPF stub-network links, so the leaf subnets a
router advertises (passive LAN segments, stub links) are invisible: users see the
transit backbone but not the networks hanging off each router, even though the
data is already in the link-state database the console fetches.

## What Changes

- `buildOspfGraph` gains stub-network handling: a router-LSA "Stub Network" link
  becomes a leaf vertex labeled with its subnet prefix (e.g. `10.0.24.0/24`),
  connected to the advertising router by an edge labeled with the link cost on
  the router side, like transit and point-to-point edges.
- All /32 stub links are hidden unconditionally: no toggle, no option. A /32
  stub is virtually always a loopback whose address duplicates the router-id
  vertex label, so showing it is pure clutter.
- A stub subnet advertised by several routers (or in several areas) merges into
  one shared vertex with one edge per advertising router.
- When the same subnet appears both as a transit network and as a stub, the
  vertex keeps its transit identity.
- The viewer shows stub vertices with their own type, legend chip, color and
  detail-pane role ("Stub network"), consistent with existing vertex kinds.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `ospf-visualization`: the "Area view content" requirement broadens its vertex
  enumeration from routers + transit subnets to also include stub-network
  subnets, with scenarios for the /32 hiding rule and the shared-stub merge;
  the "Point-to-point edge" scenario is clarified so the adjacency's direct
  edge and a separately advertised stub vertex for the same pair can coexist
  (a point-to-point adjacency itself still creates no network vertex).

## Impact

- `internal/console/ospf.go`: parse `networkAddress`/`networkMask` (dotted
  string mask) from stub links, extend `buildOspfGraph`, extend the detail
  header of `ospfView`.
- `internal/console/graph.go`: `nodeColor` branch for the new vertex type.
- `internal/console/ospf_test.go`: new stub cases; the `ospf_abr` fixtures
  already contain a shared /24 stub (R2/R4), the `ospf_r1` fixtures stay
  byte-identical as a regression anchor.
- No server, proto, or FRR command changes: stub links already arrive in the
  `show ip ospf database router json` payload.
