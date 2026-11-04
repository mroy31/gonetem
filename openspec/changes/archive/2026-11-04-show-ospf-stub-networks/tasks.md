# Tasks

## 1. Graph building for stub links

- [x] 1.1 Extend `ospfRouterLink` with `networkAddress`/`networkMask` (dotted
  string) fields and add a stub-label helper using
  `netip.ParsePrefix(addr + "(" + mask + ")")` + `Masked()`, falling back to the
  bare address on parse failure; verify with unit tests covering
  `10.0.24.0`/`255.255.255.0` -> `10.0.24.0/24` and an unparsable mask.
- [x] 1.2 In `buildOspfGraph`, pre-create transit vertices from the collected
  `transitName` map before processing links, then handle `Stub Network` links:
  drop /32 prefixes, intern the vertex as type `ospf.stub`, dedupe per
  (router, subnet) like `transitSeen`, and add the edge with `tos0Metric` as
  the router-side label; verify with unit tests for: stub subnet displayed with
  its cost, /32 stubs absent, the `ospf_abr` fixture's shared 10.0.24.0/24
  merged into one vertex with edges from 2.2.2.2 and 4.4.4.4 alongside their
  direct p2p edge, and a transit-typed subnet keeping transit typing when also
  advertised as a stub (synthesized fixture).

## 2. Viewer identity for stub vertices

- [x] 2.1 Add an `ospf.stub` -> cyan branch to `nodeColor` and make the
  `ospfView` detail header render `Stub network <name>` with its edge list;
  verify by asserting the vertex type and role line in `OspfAreaViewText`
  output for the `ospf_abr` fixtures and a synthesized /32 case in unit tests.

## 3. Integration

- [x] 3.1 Update `TestBuildOspfGraph_AbrFixtures` and area-view text
  expectations for the new 10.0.24.0/24 vertex and edges, and pin the
  `ospf_r1` view as byte-identical to its pre-change output; verify with
  `go test ./internal/console/...`.
- [x] 3.2 Verify the whole tree still builds and tests pass: `make build-amd64`
  and `go test ./...`.
  Verification note (accepted by user 2026-11-04): `make build-amd64` passes;
  `go test ./...` is red only from failures identical to the clean-HEAD
  baseline (docker/OVS/root-gated tests, a pre-existing `internal/utils` vet
  error, 5 pre-existing console render/progress tests). The failure set is
  unchanged by this change and all 27 OSPF tests pass.

## Workflow follow-up

- Archive the change after review (`/opsx-archive`).
