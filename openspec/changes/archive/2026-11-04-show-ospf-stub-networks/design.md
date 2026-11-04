# Design

## Context

`buildOspfGraph` in `internal/console/ospf.go` already receives the full
router-LSA JSON (`show ip ospf database router json`) via `ospfViewExec`; stub
links are present in that payload and are currently dropped in the per-link
switch. The two FRR JSON shapes differ in mask encoding: network LSAs carry
`networkMask` as an integer prefix length, while stub links carry it as a
dotted string (`"255.255.255.0"`). Vertices are interned through the shared
`vertex(name, typ)` helper, so equal labels merge into one node and the type of
the first registration wins. Layout is BFS-layered Sugiyama, which places leaf
vertices one layer below their parent without extra work. See proposal.md - Why
and the spec delta for the required behavior.

## Goals / Non-Goals

**Goals:**
- Render stub links as leaf vertices with minimal new machinery in the existing
  builder switch.
- Keep views without stub links (e.g. the `ospf_r1` fixtures) byte-identical.

**Non-Goals:**
- No new server/proto/exec round-trips; no reading other LSDB sections.
- No per-viewer stub visibility options (explicitly excluded by the spec).
- No change to the topology (non-OSPF) graph view.

## Decisions

- **Parse stub labels from the link fields, no extra fetch.** Extend
  `ospfRouterLink` with `NetworkAddress string` and `NetworkMask string`; the
  label comes from `net.ParseIP` on both fields, the dotted mask used as an
  `net.IPMask` via `Size()` for the prefix length:
  `v4.Mask(ipMask).String() + "/" + ones`. Alternative considered: reusing
  the network-LSA int-mask path — rejected, the JSON carries a dotted string
  and inventing a conversion adds no value. (`netip.ParsePrefix` and the
  parenthesized `net.ParseCIDR` forms looked like the fit but the latter was
  removed from Go in 1.24 and the former never accepted it; found during
  apply.) A zero/non-contiguous mask degrades like a parse failure: bare
  address label, never hidden.
- **/32 filter on prefix length.** A stub link is dropped when its parsed
  prefix length is 32. The rule keys on the prefix, not on "address equals the
  advertising router-id": FRR loopbacks are /32 whether or not they equal the
  router-id, and one numeric test is more predictable than a compound one.
  Unparsable masks therefore show the stub with the degraded bare-address
  label rather than being silently hidden.
- **Process transit links before stub links, globally.** Within the existing
  per-area/per-LSA pass this is already true because the link map is iterated
  with `sortedKeys` — but "transit wins" must hold across LSAs, so the transit
  vertices are pre-created from the already-collected `transitName` map (or an
  equivalent pre-pass) before any stub edge is added. Since vertices intern by
  label, a stub seen first in raw iteration order would otherwise steal the
  type/color of a subnet that is transit somewhere in the database.
- **One vertex per subnet label, one edge per advertiser.** Reuse `vertex()`
  interning plus a `stubSeen[[2]string{owner, subnet}]` guard mirroring
  `transitSeen`, so multi-advertiser and multi-area repeats collapse. The p2p
  edge for the same pair is untouched, producing the LSDB-faithful triangle
  (spec: "Point-to-point edge" scenario).
- **Type string `ospf.stub`, cyan.** `nodeColor` gains an explicit
  `ospf.stub` branch on `lipgloss.Color("6")`; cyan is the current default and
  is otherwise unused (routers blue, transit yellow), so the legend chip
  "stub" reads consistently with the default-styled unknowns. The `ospfView`
  detail header maps `ospf.stub` to the role "Stub network".
- **Cost label like every other edge.** The router-side edge label is
  `tos0Metric`; the network-side label is empty, as for transit edges.

## Risks / Trade-offs

- [Busy routers add leaf clutter] → the /32 rule removes the loopback mass;
  remaining stub LANs are the informative payload. If real networks prove too
  busy, a filter policy is a follow-up change, not a redesign.
- [Vertex intern by label merges a stub and a transit with equal subnet text
  that are genuinely distinct in some exotic LSDB] → acceptable: same subnet
  prefix is the same protocol identity, which is exactly the view's contract.
- [Fixture drift: adding stubs changes the `ospf_abr` canvas assertions] →
  update `TestBuildOspfGraph_AbrFixtures`/area-view text expectations once; the
  `ospf_r1` fixtures (no stub links) stay as the byte-identical anchor.
